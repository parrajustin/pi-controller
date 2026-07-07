#!/bin/bash
set -e

echo "Starting display_server..."
exec /app/display_server -dir /app/public -port 8080 -receiver /app/receiver
