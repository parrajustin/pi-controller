#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

SKIP_SETUP="false"
for arg in "$@"; do
    if [ "$arg" == "--skip_setup" ]; then
        SKIP_SETUP="true"
    fi
done

echo "Building the docker image..."
docker build -t lounge_display --build-context token_receiver=../token_receiver .

HOST_IP=$(ip -4 route get 8.8.8.8 2>/dev/null | awk '{print $7}' | tr -d '\n')
if [ -z "$HOST_IP" ]; then
    HOST_IP="127.0.0.1"
fi

echo "Detected Host IP: $HOST_IP"

# Ensure the oauth_test directory exists before mounting so it's not created as root
mkdir -p oauth_test

echo "Starting the container on port 8080 with /oauth volume mounted..."
docker run --rm -p 8080:8080 -e SKIP_SETUP="$SKIP_SETUP" -e HOST_IP="$HOST_IP" -e TOKEN_ENCRYPTION_KEY="default_local_dev_key_change_me_in_prod" -v "$(pwd)/oauth_test:/oauth" lounge_display
