#!/bin/bash
# Train YOLOv8 model on RTX 5070 Ti (or any NVIDIA GPU)
# Run from the detection/ directory: bash scripts/train.sh

set -e

DATASET_DIR="$(cd "$(dirname "$0")/.." && pwd)/dataset"
MODEL_SIZE="${1:-s}"  # n=nano, s=small, m=medium, l=large, x=extra-large
EPOCHS="${2:-100}"
IMG_SIZE="${3:-640}"
BATCH="${4:-16}"

echo "=== Cold Storage YOLOv8 Training ==="
echo "Dataset:  $DATASET_DIR"
echo "Model:    yolov8${MODEL_SIZE}.pt"
echo "Epochs:   $EPOCHS"
echo "Img Size: $IMG_SIZE"
echo "Batch:    $BATCH"
echo ""

# Check dataset exists
if [ ! -f "$DATASET_DIR/data.yaml" ]; then
    echo "ERROR: $DATASET_DIR/data.yaml not found"
    echo "Make sure you have:"
    echo "  dataset/data.yaml"
    echo "  dataset/train/images/*.jpg"
    echo "  dataset/train/labels/*.txt"
    echo "  dataset/val/images/*.jpg"
    echo "  dataset/val/labels/*.txt"
    exit 1
fi

TRAIN_COUNT=$(find "$DATASET_DIR/train/images" -type f 2>/dev/null | wc -l)
VAL_COUNT=$(find "$DATASET_DIR/val/images" -type f 2>/dev/null | wc -l)
echo "Training images: $TRAIN_COUNT"
echo "Validation images: $VAL_COUNT"
echo ""

if [ "$TRAIN_COUNT" -lt 10 ]; then
    echo "ERROR: Need at least 10 training images. Found $TRAIN_COUNT."
    exit 1
fi

# Install ultralytics if not present
pip install -q ultralytics 2>/dev/null

# Train
echo "Starting training..."
yolo detect train \
    model="yolov8${MODEL_SIZE}.pt" \
    data="$DATASET_DIR/data.yaml" \
    epochs="$EPOCHS" \
    imgsz="$IMG_SIZE" \
    batch="$BATCH" \
    project="runs/detect" \
    name="cold_storage" \
    exist_ok=true \
    patience=20 \
    save=true \
    save_period=10 \
    plots=true \
    verbose=true

echo ""
echo "=== Training Complete ==="
echo "Best model: runs/detect/cold_storage/weights/best.pt"
echo ""

# Validate
echo "Running validation..."
yolo detect val \
    model="runs/detect/cold_storage/weights/best.pt" \
    data="$DATASET_DIR/data.yaml" \
    imgsz="$IMG_SIZE" \
    verbose=true

echo ""
echo "=== Export to ONNX (portable) ==="
yolo export \
    model="runs/detect/cold_storage/weights/best.pt" \
    format=onnx \
    imgsz="$IMG_SIZE"

echo ""
echo "Done. Copy the model to the server:"
echo "  scp runs/detect/cold_storage/weights/best.pt cold@103.55.63.238:/path/to/models/"
echo "  scp runs/detect/cold_storage/weights/best.onnx cold@103.55.63.238:/path/to/models/"
