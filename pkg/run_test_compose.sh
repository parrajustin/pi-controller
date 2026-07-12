#!/bin/bash

# Ensure we're running from the directory where the script is located
cd "$(dirname "$0")"

export RECORD_SCREENS="false"

# Parse arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --record_screens) RECORD_SCREENS="true" ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

# Create necessary directories
mkdir -p ./kiosk_debug/chrome-data
chmod 777 ./kiosk_debug/chrome-data
mkdir -p ./kiosk_debug/chrome-data-2
chmod 777 ./kiosk_debug/chrome-data-2
mkdir -p ./kiosk_debug/recordings
chmod 777 ./kiosk_debug/recordings
mkdir -p ./lounge_display/oauth_test
mkdir -p ./lounge_display/logs

export HOST_IP=$(ip -4 route get 8.8.8.8 2>/dev/null | awk '{print $7}' | tr -d '\n')
if [ -z "$HOST_IP" ]; then
    export HOST_IP="127.0.0.1"
fi

# Build and run docker-compose
echo "Bringing up docker compose with Host IP $HOST_IP..."
docker compose -f docker-compose-test.yaml up --build
