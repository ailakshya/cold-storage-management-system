"""Camera stream reader — reads RTSP frames in a background thread."""

import logging
import threading
import time

import cv2
import numpy as np

logger = logging.getLogger(__name__)


class CameraStream:
    """Reads an RTSP or video stream in a background thread.

    Always holds the latest frame — never queues/buffers old frames.
    This ensures the detection pipeline always processes the most recent image.
    """

    def __init__(self, gate_id: str, url: str, target_fps: int = 10):
        self.gate_id = gate_id
        self.url = url
        self.target_fps = target_fps
        self._frame: np.ndarray | None = None
        self._lock = threading.Lock()
        self._running = False
        self._thread: threading.Thread | None = None
        self._reconnect_delay = 5  # seconds between reconnect attempts
        self._consecutive_failures = 0

    def start(self):
        self._running = True
        self._thread = threading.Thread(target=self._read_loop, daemon=True)
        self._thread.start()
        logger.info("[%s] Camera stream started: %s", self.gate_id, self.url)

    def stop(self):
        self._running = False
        if self._thread:
            self._thread.join(timeout=5)
        logger.info("[%s] Camera stream stopped", self.gate_id)

    def get_frame(self) -> np.ndarray | None:
        """Get the latest frame (non-blocking). Returns None if no frame available."""
        with self._lock:
            return self._frame.copy() if self._frame is not None else None

    def _read_loop(self):
        """Background loop: connect to stream, read frames continuously."""
        while self._running:
            cap = cv2.VideoCapture(self.url)
            if not cap.isOpened():
                self._consecutive_failures += 1
                delay = min(self._reconnect_delay * self._consecutive_failures, 60)
                logger.warning(
                    "[%s] Failed to open stream (attempt %d), retrying in %ds: %s",
                    self.gate_id, self._consecutive_failures, delay, self.url,
                )
                time.sleep(delay)
                continue

            self._consecutive_failures = 0
            logger.info("[%s] Connected to camera stream", self.gate_id)

            frame_interval = 1.0 / self.target_fps
            last_read = 0.0

            while self._running:
                now = time.time()
                # Throttle to target FPS
                if now - last_read < frame_interval:
                    # Grab (but don't decode) to keep stream position current
                    cap.grab()
                    time.sleep(0.001)
                    continue

                ret, frame = cap.read()
                if not ret:
                    logger.warning("[%s] Lost camera stream, reconnecting...", self.gate_id)
                    break

                with self._lock:
                    self._frame = frame
                last_read = now

            cap.release()

            if self._running:
                time.sleep(self._reconnect_delay)
