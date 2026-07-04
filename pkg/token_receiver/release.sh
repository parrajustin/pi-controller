#!/bin/bash

set -e

# Change to the directory of the script
cd "$(dirname "$0")"

# File containing the version
VERSION_FILE="version.json"

# Check if version.json exists, if not initialize it
if [ ! -f "$VERSION_FILE" ]; then
  echo '{"version": 0}' > "$VERSION_FILE"
fi

# Read current version
CURRENT_VERSION=$(jq -r '.version' "$VERSION_FILE")

# Increment version
NEW_VERSION=$((CURRENT_VERSION + 1))

# Update version.json
jq --arg v "$NEW_VERSION" '.version = ($v | tonumber)' "$VERSION_FILE" > version.tmp.json && mv version.tmp.json "$VERSION_FILE"

TAGNAME="v${NEW_VERSION}"
IMAGE_NAME="xerofuzzion/lounge_display_token_reciever"

echo "Building and pushing version ${TAGNAME}-x86_64 and latest-x86_64..."
docker buildx build --platform linux/amd64 -t "${IMAGE_NAME}:${TAGNAME}-x86_64" -t "${IMAGE_NAME}:latest-x86_64" --push .

echo "Building and pushing version ${TAGNAME}-aarch64 and latest-aarch64..."
docker buildx build --platform linux/arm64 -t "${IMAGE_NAME}:${TAGNAME}-aarch64" -t "${IMAGE_NAME}:latest-aarch64" --push .

echo "Successfully built and pushed multi-arch images for ${TAGNAME}"
echo "version.json updated to version ${NEW_VERSION}"
