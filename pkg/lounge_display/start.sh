#!/bin/bash
set -e

echo "Starting setup_server..."
exec /app/setup_server -dir /app/setup_web -port 8080

echo "Setup complete. Starting display_server..."
exec /app/display_server -dir /app/public -port 8080
