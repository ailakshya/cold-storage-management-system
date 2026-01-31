"""Bag counting logic — tracks unique bags per vehicle unloading session.
Records video proof during each session."""

import logging
import os
import time
from dataclasses import dataclass, field
from datetime import datetime

import cv2
import numpy as np
import supervision as sv

from config import (
    CLASS_POTATO_BAG,
    CLASS_BAG_CLUSTER,
    CLASS_VEHICLE,
    VEHICLE_ABSENCE_TIMEOUT,
)

logger = logging.getLogger(__name__)

VIDEO_DIR = os.environ.get("VIDEO_DIR", "/tmp/detection_videos")
os.makedirs(VIDEO_DIR, exist_ok=True)


@dataclass
class UnloadingSession:
    """Tracks one vehicle unloading session at a gate."""

    gate_id: str
    started_at: float = field(default_factory=time.time)
    last_vehicle_seen: float = field(default_factory=time.time)
    unique_bag_ids: set = field(default_factory=set)
    bag_cluster_count: int = 0
    peak_bags_in_frame: int = 0
    total_frames: int = 0
    vehicle_confidence_sum: float = 0.0
    vehicle_detections: int = 0
    ended: bool = False

    # Video recording
    video_writer: object = field(default=None, repr=False)
    video_path: str = ""
    video_size_bytes: int = 0

    @property
    def bag_count(self) -> int:
        return len(self.unique_bag_ids)

    @property
    def estimated_total(self) -> int:
        """Individual tracked bags + estimated bags from clusters."""
        return self.bag_count + self.bag_cluster_count

    @property
    def avg_vehicle_confidence(self) -> float:
        if self.vehicle_detections == 0:
            return 0.0
        return self.vehicle_confidence_sum / self.vehicle_detections

    @property
    def duration_seconds(self) -> float:
        end = self.last_vehicle_seen if self.ended else time.time()
        return end - self.started_at

    def start_recording(self, frame_width: int, frame_height: int, fps: float = 10.0):
        """Start recording video for this session."""
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        self.video_path = os.path.join(VIDEO_DIR, f"{self.gate_id}_{ts}.mp4")
        fourcc = cv2.VideoWriter_fourcc(*"mp4v")
        self.video_writer = cv2.VideoWriter(
            self.video_path, fourcc, fps, (frame_width, frame_height)
        )
        logger.info("[%s] Recording video: %s", self.gate_id, self.video_path)

    def write_frame(self, frame):
        """Write a frame to the video file."""
        if self.video_writer is not None and self.video_writer.isOpened():
            self.video_writer.write(frame)

    def stop_recording(self):
        """Stop recording and finalize video file."""
        if self.video_writer is not None:
            self.video_writer.release()
            self.video_writer = None
        if self.video_path and os.path.exists(self.video_path):
            self.video_size_bytes = os.path.getsize(self.video_path)
            logger.info(
                "[%s] Video saved: %s (%.1f MB)",
                self.gate_id,
                self.video_path,
                self.video_size_bytes / 1024 / 1024,
            )

    def to_dict(self) -> dict:
        return {
            "gate_id": self.gate_id,
            "started_at": self.started_at,
            "duration_seconds": round(self.duration_seconds, 1),
            "bag_count": self.bag_count,
            "bag_cluster_count": self.bag_cluster_count,
            "estimated_total": self.estimated_total,
            "peak_bags_in_frame": self.peak_bags_in_frame,
            "total_frames": self.total_frames,
            "avg_vehicle_confidence": round(self.avg_vehicle_confidence, 3),
            "video_path": self.video_path,
            "video_size_bytes": self.video_size_bytes,
        }


class BagCounter:
    """Per-gate bag counter using ByteTrack for object tracking."""

    def __init__(self, gate_id: str):
        self.gate_id = gate_id
        self.tracker = sv.ByteTrack(
            track_activation_threshold=0.3,
            lost_track_buffer=60,  # keep lost tracks for 60 frames (~6s at 10fps)
            minimum_matching_threshold=0.8,
            frame_rate=10,
        )
        self.session: UnloadingSession | None = None

    def process_frame(self, detections: list[dict], frame=None) -> dict | None:
        """Process detections for one frame.

        Args:
            detections: List of detection dicts from the detector.
            frame: Raw BGR frame for video recording (optional).

        Returns a completed session dict when the vehicle leaves, or None.
        """
        bags = [d for d in detections if d["class_id"] == CLASS_POTATO_BAG]
        clusters = [d for d in detections if d["class_id"] == CLASS_BAG_CLUSTER]
        vehicles = [d for d in detections if d["class_id"] == CLASS_VEHICLE]

        now = time.time()
        completed_session = None

        # Vehicle present — start or continue session
        if vehicles:
            if self.session is None:
                logger.info("[%s] Vehicle detected — starting unloading session", self.gate_id)
                self.session = UnloadingSession(gate_id=self.gate_id)
                # Start video recording if frame is available
                if frame is not None:
                    h, w = frame.shape[:2]
                    self.session.start_recording(w, h)

            self.session.last_vehicle_seen = now
            self.session.vehicle_detections += 1
            self.session.vehicle_confidence_sum += max(v["confidence"] for v in vehicles)

        # No vehicle — check if session should end
        elif self.session is not None:
            elapsed = now - self.session.last_vehicle_seen
            if elapsed > VEHICLE_ABSENCE_TIMEOUT:
                self.session.ended = True
                self.session.stop_recording()
                completed_session = self.session.to_dict()
                logger.info(
                    "[%s] Session ended — %d bags counted (%d tracked + %d clusters) in %.0fs",
                    self.gate_id,
                    self.session.estimated_total,
                    self.session.bag_count,
                    self.session.bag_cluster_count,
                    self.session.duration_seconds,
                )
                self.session = None
                self.tracker = sv.ByteTrack(
                    track_activation_threshold=0.3,
                    lost_track_buffer=60,
                    minimum_matching_threshold=0.8,
                    frame_rate=10,
                )
                return completed_session

        # Record frame to video if session active
        if self.session is not None and frame is not None:
            self.session.write_frame(frame)

        # Track individual bags if session is active
        if self.session is not None and bags:
            # Build supervision Detections object for tracking
            xyxy = np.array([d["bbox"] for d in bags], dtype=np.float32)
            confidences = np.array([d["confidence"] for d in bags], dtype=np.float32)
            class_ids = np.array([d["class_id"] for d in bags], dtype=int)

            sv_detections = sv.Detections(
                xyxy=xyxy,
                confidence=confidences,
                class_id=class_ids,
            )

            # Run ByteTrack
            tracked = self.tracker.update_with_detections(sv_detections)

            # Record unique tracker IDs
            if tracked.tracker_id is not None:
                for tid in tracked.tracker_id:
                    self.session.unique_bag_ids.add(int(tid))

            self.session.total_frames += 1
            bags_this_frame = len(bags)
            if bags_this_frame > self.session.peak_bags_in_frame:
                self.session.peak_bags_in_frame = bags_this_frame

        # Count clusters (not individually trackable)
        if self.session is not None and clusters:
            cluster_count = len(clusters)
            if cluster_count > self.session.bag_cluster_count:
                self.session.bag_cluster_count = cluster_count

        return completed_session
