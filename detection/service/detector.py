"""YOLOv8 detector — loads model once, runs inference on frames."""

import logging
from ultralytics import YOLO
import numpy as np
from config import MODEL_PATH, MODEL_CONFIDENCE, MODEL_IOU_THRESHOLD, MODEL_IMG_SIZE

logger = logging.getLogger(__name__)


class Detector:
    """Wraps YOLOv8 model for potato bag + vehicle detection."""

    def __init__(self):
        logger.info("Loading YOLOv8 model from %s", MODEL_PATH)
        self.model = YOLO(MODEL_PATH)
        logger.info("Model loaded: %s", self.model.model_name if hasattr(self.model, 'model_name') else MODEL_PATH)

    def detect(self, frame: np.ndarray) -> list[dict]:
        """Run detection on a single frame.

        Returns list of detections:
            [{"class_id": int, "class_name": str, "confidence": float,
              "bbox": [x1, y1, x2, y2], "bbox_norm": [cx, cy, w, h]}]
        """
        results = self.model.predict(
            frame,
            conf=MODEL_CONFIDENCE,
            iou=MODEL_IOU_THRESHOLD,
            imgsz=MODEL_IMG_SIZE,
            verbose=False,
        )

        detections = []
        if not results or len(results) == 0:
            return detections

        result = results[0]
        if result.boxes is None:
            return detections

        h, w = frame.shape[:2]

        for box in result.boxes:
            cls_id = int(box.cls[0])
            conf = float(box.conf[0])
            x1, y1, x2, y2 = box.xyxy[0].tolist()

            # Normalized center-x, center-y, width, height (YOLO format)
            cx = ((x1 + x2) / 2) / w
            cy = ((y1 + y2) / 2) / h
            bw = (x2 - x1) / w
            bh = (y2 - y1) / h

            detections.append({
                "class_id": cls_id,
                "class_name": self.model.names.get(cls_id, f"class_{cls_id}"),
                "confidence": round(conf, 3),
                "bbox": [round(v, 1) for v in [x1, y1, x2, y2]],
                "bbox_norm": [round(v, 4) for v in [cx, cy, bw, bh]],
            })

        return detections
