#!/bin/bash

# Ensure we're running from the directory where the script is located
cd "$(dirname "$0")"

KIOSK_DIR="."
IMAGE_NAME="kiosk_debug_img"
CONTAINER_NAME="kiosk_debug"
CHROME_DATA_DIR="$(pwd)/chrome-data"

# Create chrome-data directory if it doesn't exist
echo "Setting up persistent chrome data directory at $CHROME_DATA_DIR"
mkdir -p "$CHROME_DATA_DIR"

# Make sure it's writable by the container's Chromium user
chmod 777 "$CHROME_DATA_DIR"

echo "Building Docker image..."
docker build -t "$IMAGE_NAME" "./$KIOSK_DIR"

# Stop and remove existing container if it exists
if [ "$(docker ps -aq -f name=^${CONTAINER_NAME}$)" ]; then
    echo "Removing existing container..."
    docker rm -f "$CONTAINER_NAME"
fi

echo ""
echo "================================================="
echo "Kiosk debug container will run soon!"
echo "================================================="
echo " - noVNC Browser Access : http://localhost:5050"
echo " - Remote Debugging     : http://localhost:9222"
echo " - Native VNC           : localhost:5900"
echo "Chrome data is persisted to: $CHROME_DATA_DIR"
echo "To view logs, run: docker logs -f $CONTAINER_NAME"
docker run \
    --name "$CONTAINER_NAME" \
    --restart always \
    --privileged \
    -p 5050:5050 \
    -p 9222:9222 \
    -p 5900:5900 \
    -v "$CHROME_DATA_DIR:/chrome-data" \
    "$IMAGE_NAME"

