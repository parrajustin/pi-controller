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

# 1. make folder and delete all contents of "dist" if there is any
rm -rf dist
mkdir -p dist

# 2. make folder "pkg" in "dist"
mkdir -p dist/pkg

# 3. Create dirs and Copy all data listed in "config.json" to "pkg"
echo "Copying files based on config.json to pkg..."
TOP_LEVEL_ITEMS=$(jq -r '.update_files[]?, .update_directories[]?' config.json | cut -d'/' -f1 | sort | uniq)

for item in $TOP_LEVEL_ITEMS; do
  # we copy all data except for the binary data.
  if [ "$item" != "pi-controller" ] && [ "$item" != "updater" ] && [ "$item" != "runner" ] && [ "$item" != "version.json" ]; then
    if [ -f "$item" ]; then
      cp "$item" dist/pkg/
    elif [ -d "$item" ]; then
      cp -r "$item" dist/pkg/
    fi
  fi
done

cp version.json dist/pkg/

# 4. build all binaries to "dist" for arch and x86 and whatnot
echo "Building binaries for all architectures to dist/..."

# aarch64
GOOS=linux GOARCH=arm64 go build -o dist/pi-controller-aarch64 ./cmd/pi-controller
GOOS=linux GOARCH=arm64 go build -o dist/updater-aarch64 ./cmd/updater
GOOS=linux GOARCH=arm64 go build -o dist/runner-aarch64 ./cmd/runner

# x86_64
GOOS=linux GOARCH=amd64 go build -o dist/pi-controller-x86_64 ./cmd/pi-controller
GOOS=linux GOARCH=amd64 go build -o dist/updater-x86_64 ./cmd/updater
GOOS=linux GOARCH=amd64 go build -o dist/runner-x86_64 ./cmd/runner

# 5. For each arch
for ARCH in aarch64 x86_64; do
  echo "--- Packaging $ARCH ---"
  
  # copy the binaries to "pkg" with the correct name
  cp dist/pi-controller-${ARCH} dist/pkg/pi-controller
  cp dist/updater-${ARCH} dist/pkg/updater
  cp dist/runner-${ARCH} dist/pkg/runner
  
  # run the release validator for all the files to make sure we have them all
  echo "Verifying files against config.json for $ARCH..."
  go run scripts/validate_release.go dist/pkg
  
  # create the .tar.gz and .sig to the /dist folder
  echo "Packaging release for $ARCH..."
  TARBALL="dist/pi-controller-${ARCH}.tar.gz"
  SIGNATURE="dist/pi-controller-${ARCH}.tar.gz.sig"
  
  tar -czvf "$TARBALL" -C dist/pkg $TOP_LEVEL_ITEMS
  
  echo "Signing release with privatekey.pem..."
  openssl dgst -sha256 -sign privatekey.pem -out "$SIGNATURE" "$TARBALL"
done

echo "Cleaning up temporary files from dist folder..."
rm -rf dist/pkg
rm -f dist/pi-controller-aarch64 dist/updater-aarch64 dist/runner-aarch64
rm -f dist/pi-controller-x86_64 dist/updater-x86_64 dist/runner-x86_64

echo "Release $VERSION packaged and signed successfully!"

# 6. once all archs are done create the git release
if command -v gh >/dev/null 2>&1; then
  echo "Creating GitHub release..."
  gh release create "$VERSION" "dist/pi-controller-aarch64.tar.gz" "dist/pi-controller-aarch64.tar.gz.sig" "dist/pi-controller-x86_64.tar.gz" "dist/pi-controller-x86_64.tar.gz.sig" --title "Release $VERSION" --notes "Release $VERSION auto-generated"
  echo "Release uploaded successfully!"
else
  echo "GitHub CLI (gh) not found. Please upload the tarballs and signatures in the dist/ folder to GitHub releases manually with tag $VERSION."
fi
