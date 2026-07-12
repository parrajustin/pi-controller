#!/bin/bash
set -e

echo "Starting pi-controller installation script..."

# Require root privileges to setup systemd service
echo "Step 1: Checking for root privileges (required to install the systemd service)..."
if [ "$EUID" -ne 0 ]; then
  echo "ERROR: This script must be run as root."
  echo "Please run using sudo: sudo ./install.sh"
  exit 1
fi
echo "Root privileges confirmed."

echo "Step 2: Detecting system architecture to download the correct binary..."
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)
        TAR_ARCH="x86_64"
        echo "Architecture detected as x86_64."
        ;;
    aarch64|arm64)
        TAR_ARCH="aarch64"
        echo "Architecture detected as ARM64 (aarch64)."
        ;;
    *)
        echo "ERROR: Unsupported architecture detected: $ARCH"
        exit 1
        ;;
esac

echo "Step 3: Querying GitHub API for the latest release information..."
LATEST_RELEASE_API="https://api.github.com/repos/parrajustin/pi-controller/releases/latest"
DOWNLOAD_URL=$(curl -s "$LATEST_RELEASE_API" | grep -o "https://github.com/parrajustin/pi-controller/releases/download/[^\"]*pi-controller-${TAR_ARCH}\.tar\.gz" | head -n 1)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "ERROR: Could not find a valid release URL for architecture $TAR_ARCH"
    exit 1
fi

echo "Found release URL: $DOWNLOAD_URL"
echo "Step 4: Downloading release tarball from GitHub..."
curl -L -o release.tar.gz "$DOWNLOAD_URL"

echo "Step 5: Extracting the downloaded release tarball (release.tar.gz)..."
tar -xzf release.tar.gz

echo "Step 6: Cleaning up the downloaded tarball..."
rm release.tar.gz
echo "Extraction and cleanup complete."

CURRENT_DIR=$(pwd)
SERVICE_NAME="pi-controller.service"

echo "Step 7: Setting up the systemd service."
echo "The binary will be run from the current directory: $CURRENT_DIR"
echo "Generating systemd service file at /etc/systemd/system/${SERVICE_NAME}..."

# Create systemd service file dynamically to use the current directory
cat <<EOF > /etc/systemd/system/${SERVICE_NAME}
[Unit]
Description=Pi Controller Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${CURRENT_DIR}
ExecStart=${CURRENT_DIR}/pi-controller
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

echo "Systemd service file created successfully."

echo "Step 8: Reloading systemd daemon to recognize the new service..."
systemctl daemon-reload

echo "Step 9: Enabling the ${SERVICE_NAME} to start automatically on boot..."
systemctl enable ${SERVICE_NAME}

echo "Step 10: Starting the ${SERVICE_NAME} right now..."
systemctl start ${SERVICE_NAME}

echo "============================================================"
echo "Installation and setup complete!"
echo "The pi-controller service is now running in the background."
echo "You can check its status at any time with: systemctl status ${SERVICE_NAME}"
echo "You can view its logs with: journalctl -u ${SERVICE_NAME} -f"
echo "============================================================"
