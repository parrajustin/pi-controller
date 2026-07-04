#!/bin/bash
set -e

echo "Starting setup_server..."
until /app/setup_server -dir /app/setup_web -port 8080; do
  echo "setup_server failed. Restarting in 1 second..."
  sleep 1
done

echo "Setup complete. Starting display_server..."
exec /app/display_server -dir /app/public -port 8080
