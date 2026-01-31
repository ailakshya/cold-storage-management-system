"""Detection service configuration — loaded from environment variables."""

import os

# Model
MODEL_PATH = os.getenv("MODEL_PATH", "/models/best.pt")
MODEL_CONFIDENCE = float(os.getenv("MODEL_CONFIDENCE", "0.35"))
MODEL_IOU_THRESHOLD = float(os.getenv("MODEL_IOU_THRESHOLD", "0.45"))
MODEL_IMG_SIZE = int(os.getenv("MODEL_IMG_SIZE", "640"))

# Camera RTSP streams (comma-separated)
# Format: gate_id:rtsp_url,gate_id:rtsp_url
# Example: gate1:rtsp://192.168.1.10:554/stream1,gate2:rtsp://192.168.1.11:554/stream1
CAMERA_URLS = os.getenv("CAMERA_URLS", "")

# Backend API
BACKEND_URL = os.getenv("BACKEND_URL", "http://cold-backend:8080")
BACKEND_API_KEY = os.getenv("BACKEND_API_KEY", "")

# Processing
INFERENCE_FPS = int(os.getenv("INFERENCE_FPS", "10"))
VEHICLE_ABSENCE_TIMEOUT = int(os.getenv("VEHICLE_ABSENCE_TIMEOUT", "30"))  # seconds without vehicle = session ends
MIN_BAG_COUNT_TO_REPORT = int(os.getenv("MIN_BAG_COUNT_TO_REPORT", "1"))

# Class IDs (must match data.yaml)
CLASS_POTATO_BAG = 0
CLASS_BAG_CLUSTER = 1
CLASS_VEHICLE = 2

CLASS_NAMES = {
    CLASS_POTATO_BAG: "potato_bag",
    CLASS_BAG_CLUSTER: "bag_cluster",
    CLASS_VEHICLE: "vehicle",
}
