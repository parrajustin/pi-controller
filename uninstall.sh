#!/bin/bash
set -e

echo "Starting pi-controller uninstallation script..."

if [ "$EUID" -ne 0 ]; then
  echo "ERROR: This script must be run as root."
  echo "Please run using sudo: sudo ./uninstall.sh"
  exit 1
fi

SERVICE_NAME="pi-controller.service"
ENV_FILE="/etc/pi-controller.env"
CURRENT_DIR=$(pwd)

echo "Step 1: Stopping the systemd service..."
systemctl stop ${SERVICE_NAME} || true

echo "Step 2: Disabling the systemd service..."
systemctl disable ${SERVICE_NAME} || true

echo "Step 3: Taking down docker containers..."
if [ -f "$CURRENT_DIR/docker/docker-compose.yml" ]; then
    cd "$CURRENT_DIR"
    docker compose -f docker/docker-compose.yml down || true
else
    echo "docker-compose.yml not found, skipping docker compose down."
fi

echo "Step 4: Removing systemd service file..."
if [ -f "/etc/systemd/system/${SERVICE_NAME}" ]; then
    rm "/etc/systemd/system/${SERVICE_NAME}"
fi

echo "Step 5: Removing environment configuration..."
if [ -f "${ENV_FILE}" ]; then
    rm "${ENV_FILE}"
fi

echo "Step 6: Reloading systemd daemon..."
systemctl daemon-reload

echo "============================================================"
echo "Uninstallation complete!"
echo "============================================================"
