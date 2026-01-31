"""Cold Storage Detection Service — main entry point.

Reads RTSP camera streams, runs YOLOv8 inference, tracks and counts
potato bags per vehicle unloading session, reports results to Go backend.
"""

import json
import logging
import signal
import sys
import time

import requests

from camera import CameraStream
from config import (
    BACKEND_API_KEY,
    BACKEND_URL,
    CAMERA_URLS,
    INFERENCE_FPS,
    MIN_BAG_COUNT_TO_REPORT,
)
from counter import BagCounter
from detector import Detector

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger("detection")


def parse_camera_urls(raw: str) -> list[tuple[str, str]]:
    """Parse 'gate1:rtsp://...,gate2:rtsp://...' into [(gate_id, url)]."""
    cameras = []
    for entry in raw.split(","):
        entry = entry.strip()
        if not entry:
            continue
        if ":" in entry and entry.index(":") < entry.index("//") if "//" in entry else True:
            gate_id, url = entry.split(":", 1)
            cameras.append((gate_id.strip(), url.strip()))
        else:
            # No gate_id prefix — use index
            cameras.append((f"gate{len(cameras)+1}", entry.strip()))
    return cameras


def report_session(session: dict):
    """POST completed unloading session to Go backend."""
    if session["estimated_total"] < MIN_BAG_COUNT_TO_REPORT:
        logger.info("Session at %s had %d bags — below threshold, skipping report",
                     session["gate_id"], session["estimated_total"])
        return

    url = f"{BACKEND_URL}/api/detections"
    headers = {"Content-Type": "application/json"}
    if BACKEND_API_KEY:
        headers["Authorization"] = f"Bearer {BACKEND_API_KEY}"

    try:
        resp = requests.post(url, json=session, headers=headers, timeout=10)
        if resp.status_code in (200, 201):
            logger.info("Reported session to backend: gate=%s bags=%d",
                        session["gate_id"], session["estimated_total"])
        else:
            logger.error("Backend rejected session report: %d %s",
                         resp.status_code, resp.text[:200])
    except requests.RequestException as e:
        logger.error("Failed to report session to backend: %s", e)


def main():
    # Parse camera config
    cameras = parse_camera_urls(CAMERA_URLS)
    if not cameras:
        logger.error("No cameras configured. Set CAMERA_URLS environment variable.")
        logger.error("Format: gate1:rtsp://ip:554/stream,gate2:rtsp://ip:554/stream")
        sys.exit(1)

    logger.info("Starting detection service with %d camera(s)", len(cameras))
    for gate_id, url in cameras:
        logger.info("  %s -> %s", gate_id, url)

    # Initialize detector (loads model once)
    detector = Detector()

    # Start camera streams
    streams: list[tuple[str, CameraStream, BagCounter]] = []
    for gate_id, url in cameras:
        stream = CameraStream(gate_id, url, target_fps=INFERENCE_FPS)
        counter = BagCounter(gate_id)
        stream.start()
        streams.append((gate_id, stream, counter))

    # Graceful shutdown
    running = True

    def shutdown(signum, frame):
        nonlocal running
        logger.info("Shutting down...")
        running = False

    signal.signal(signal.SIGINT, shutdown)
    signal.signal(signal.SIGTERM, shutdown)

    logger.info("Detection pipeline running — processing %d stream(s) at %d fps",
                len(streams), INFERENCE_FPS)

    # Main loop — round-robin across all cameras
    frame_interval = 1.0 / INFERENCE_FPS
    while running:
        cycle_start = time.time()

        for gate_id, stream, counter in streams:
            frame = stream.get_frame()
            if frame is None:
                continue

            # Run YOLOv8 inference
            detections = detector.detect(frame)

            # Process through bag counter — pass frame for video recording
            completed = counter.process_frame(detections, frame=frame)

            # If a session just completed, report it
            if completed is not None:
                report_session(completed)

        # Maintain target FPS across all cameras
        elapsed = time.time() - cycle_start
        sleep_time = frame_interval - elapsed
        if sleep_time > 0:
            time.sleep(sleep_time)

    # Cleanup
    logger.info("Stopping camera streams...")
    for _, stream, _ in streams:
        stream.stop()
    logger.info("Detection service stopped")


if __name__ == "__main__":
    main()
