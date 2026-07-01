#!/bin/bash
set -e

if [ -z "$1" ]; then
  echo "Usage: $0 <version>"
  echo "Example: $0 v1.0.1"
  exit 1
fi

VERSION=$1

echo "Setting version to $VERSION..."
cat <<EOF > version.json
{
  "version": "$VERSION"
}
EOF

mkdir -p dist
cp version.json splash.png publickey.pem config.json dist/
cp -r docker dist/

# Build and package for aarch64 (arm64)
echo "Building pi-controller for linux/aarch64 (arm64)..."
GOOS=linux GOARCH=arm64 go build -o dist/pi-controller ./cmd/pi-controller
echo "Building updater for linux/aarch64 (arm64)..."
GOOS=linux GOARCH=arm64 go build -o dist/updater ./cmd/updater
echo "Building runner for linux/aarch64 (arm64)..."
GOOS=linux GOARCH=arm64 go build -o dist/runner ./cmd/runner

echo "Packaging release for aarch64..."
TARBALL_AARCH64="dist/pi-controller-aarch64.tar.gz"
tar -czvf "$TARBALL_AARCH64" -C dist pi-controller updater runner version.json splash.png publickey.pem config.json docker

echo "Signing release with privatekey.pem..."
SIGNATURE_AARCH64="dist/pi-controller-aarch64.tar.gz.sig"
openssl dgst -sha256 -sign privatekey.pem -out "$SIGNATURE_AARCH64" "$TARBALL_AARCH64"

# Build and package for x86_64 (amd64)
echo "Building pi-controller for linux/x86_64 (amd64)..."
GOOS=linux GOARCH=amd64 go build -o dist/pi-controller ./cmd/pi-controller
echo "Building updater for linux/x86_64 (amd64)..."
GOOS=linux GOARCH=amd64 go build -o dist/updater ./cmd/updater
echo "Building runner for linux/x86_64 (amd64)..."
GOOS=linux GOARCH=amd64 go build -o dist/runner ./cmd/runner

echo "Packaging release for x86_64..."
TARBALL_X86_64="dist/pi-controller-x86_64.tar.gz"
tar -czvf "$TARBALL_X86_64" -C dist pi-controller updater runner version.json splash.png publickey.pem config.json docker

echo "Signing release with privatekey.pem..."
SIGNATURE_X86_64="dist/pi-controller-x86_64.tar.gz.sig"
openssl dgst -sha256 -sign privatekey.pem -out "$SIGNATURE_X86_64" "$TARBALL_X86_64"

echo "Release $VERSION packaged and signed successfully!"

if command -v gh >/dev/null 2>&1; then
  echo "Creating GitHub release..."
  gh release create "$VERSION" "$TARBALL_AARCH64" "$SIGNATURE_AARCH64" "$TARBALL_X86_64" "$SIGNATURE_X86_64" --title "Release $VERSION" --notes "Release $VERSION auto-generated"
  echo "Release uploaded successfully!"
else
  echo "GitHub CLI (gh) not found. Please upload the tarballs and signatures in the dist/ folder to GitHub releases manually with tag $VERSION."
fi
