#!/bin/bash
set -e

# Stop any running instances just in case
docker stop iroh_token_server >/dev/null 2>&1 || true

echo "Building Docker container..."
docker build -t iroh_token_system .

echo "Starting Server in the background..."
docker run --rm -p 7070:7070 --name iroh_token_server -e SERVER_ADDR=0.0.0.0:7070 iroh_token_system /app/server
