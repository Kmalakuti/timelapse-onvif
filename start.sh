#!/bin/sh
set -eu

: "${CAMERA_NAME:?Need CAMERA_NAME}"
: "${RTSP_URL:?Need RTSP_URL}"
: "${INTERVAL_SECONDS:?Need INTERVAL_SECONDS}"
OUT_DIR="${OUT_DIR:-/data}"

mkdir -p "$OUT_DIR/$CAMERA_NAME"

# UTC timestamps in filenames (container is typically UTC already; TZ env helps too)
OUT_PATTERN="$OUT_DIR/$CAMERA_NAME/%Y%m%dT%H%M%SZ.jpg"

echo "Starting persistent FFmpeg for $CAMERA_NAME"
echo "RTSP_URL=[redacted]"
echo "INTERVAL_SECONDS=$INTERVAL_SECONDS"
echo "OUT_PATTERN=$OUT_PATTERN"

# Loop so if the camera reboots or RTSP drops, it auto-recovers.
while true; do
  echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] ffmpeg starting..."

  ffmpeg -hide_banner -loglevel warning \
    -rtsp_transport tcp \
    -rtsp_flags prefer_tcp \
    -use_wallclock_as_timestamps 1 \
    -fflags +genpts+discardcorrupt \
    -err_detect ignore_err \
    -i "$RTSP_URL" \
    -an -map 0:v:0 \
    -vf "fps=1/${INTERVAL_SECONDS}" \
    -q:v 2 \
    -f image2 -strftime 1 \
    -y "$OUT_PATTERN"

  code=$?
  echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] ffmpeg exited (code=$code). Restarting in 2s..."
  sleep 2
done
