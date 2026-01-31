#!/usr/bin/env python3
"""Extract frames from video files for dataset creation.

Usage:
    python extract_frames.py /path/to/videos/ /path/to/output/ --fps 0.5

    --fps 0.5 means 1 frame every 2 seconds (recommended for unloading videos)
"""

import argparse
import os
import sys

import cv2


def extract_frames(video_path: str, output_dir: str, fps: float, prefix: str = ""):
    cap = cv2.VideoCapture(video_path)
    if not cap.isOpened():
        print(f"  ERROR: Cannot open {video_path}")
        return 0

    video_fps = cap.get(cv2.CAP_PROP_FPS)
    if video_fps <= 0:
        video_fps = 30.0

    frame_interval = int(video_fps / fps)
    if frame_interval < 1:
        frame_interval = 1

    frame_count = 0
    saved_count = 0

    while True:
        ret, frame = cap.read()
        if not ret:
            break

        if frame_count % frame_interval == 0:
            filename = f"{prefix}frame_{saved_count:06d}.jpg"
            filepath = os.path.join(output_dir, filename)
            cv2.imwrite(filepath, frame, [cv2.IMWRITE_JPEG_QUALITY, 95])
            saved_count += 1

        frame_count += 1

    cap.release()
    return saved_count


def main():
    parser = argparse.ArgumentParser(description="Extract frames from videos for YOLOv8 dataset")
    parser.add_argument("input", help="Video file or directory of videos")
    parser.add_argument("output", help="Output directory for extracted frames")
    parser.add_argument("--fps", type=float, default=0.5, help="Frames per second to extract (default: 0.5 = 1 frame every 2s)")
    args = parser.parse_args()

    os.makedirs(args.output, exist_ok=True)

    video_extensions = {".mp4", ".mov", ".avi", ".mkv", ".webm", ".m4v", ".mts"}

    if os.path.isfile(args.input):
        videos = [args.input]
    elif os.path.isdir(args.input):
        videos = sorted([
            os.path.join(args.input, f)
            for f in os.listdir(args.input)
            if os.path.splitext(f)[1].lower() in video_extensions
        ])
    else:
        print(f"ERROR: {args.input} not found")
        sys.exit(1)

    if not videos:
        print(f"No video files found in {args.input}")
        sys.exit(1)

    print(f"Found {len(videos)} video(s), extracting at {args.fps} fps")
    total = 0

    for i, video_path in enumerate(videos):
        video_name = os.path.splitext(os.path.basename(video_path))[0]
        prefix = f"{video_name}_"
        print(f"  [{i+1}/{len(videos)}] {os.path.basename(video_path)}...", end=" ")
        count = extract_frames(video_path, args.output, args.fps, prefix)
        print(f"{count} frames")
        total += count

    print(f"\nTotal: {total} frames extracted to {args.output}")


if __name__ == "__main__":
    main()
