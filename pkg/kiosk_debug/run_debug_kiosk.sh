#!/bin/bash

# Ensure we're running from the directory where the script is located
cd "$(dirname "$0")"

KIOSK_DIR="."
IMAGE_NAME="kiosk_debug_img"
CONTAINER_NAME="kiosk_debug"
CHROME_DATA_DIR="$(pwd)/chrome-data"
CHROME_DATA_DIR_2="$(pwd)/chrome-data-2"

# Create chrome-data directory if it doesn't exist
echo "Setting up persistent chrome data directory at $CHROME_DATA_DIR"
mkdir -p "$CHROME_DATA_DIR"

# Make sure it's writable by the container's Chromium user
chmod 777 "$CHROME_DATA_DIR"

# Create chrome-data-2 directory if it doesn't exist
echo "Setting up persistent chrome data directory at $CHROME_DATA_DIR_2"
mkdir -p "$CHROME_DATA_DIR_2"

# Make sure it's writable by the container's Chromium user
chmod 777 "$CHROME_DATA_DIR_2"

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
echo "Display 1 (Google Meet):"
echo " - noVNC Browser Access : http://localhost:5050"
echo " - Remote Debugging     : http://localhost:9222"
echo " - Native VNC           : localhost:5900"
echo "Chrome data is persisted to: $CHROME_DATA_DIR"
echo "-------------------------------------------------"
echo "Display 2 (localhost:8080):"
echo " - noVNC Browser Access : http://localhost:5051"
echo " - Remote Debugging     : http://localhost:9224"
echo " - Native VNC           : localhost:5901"
echo "Chrome data is persisted to: $CHROME_DATA_DIR_2"
echo "================================================="
echo "To view logs, run: docker logs -f $CONTAINER_NAME"
docker run \
    --name "$CONTAINER_NAME" \
    --restart always \
    --privileged \
    --shm-size=2g \
    -p 5050:5050 \
    -p 5051:5051 \
    -p 9222:9223 \
    -p 9224:9225 \
    -p 5900:5900 \
    -p 5901:5901 \
    -v "$CHROME_DATA_DIR:/chrome-data" \
    -v "$CHROME_DATA_DIR_2:/chrome-data-2" \
    "$IMAGE_NAME"

