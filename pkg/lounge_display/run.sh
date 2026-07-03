#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

echo "Building the docker image..."
docker build -t lounge_display .

echo "Starting the container on port 8080..."
docker run --rm -p 8080:8080 lounge_display
