#!/bin/bash

set -e

# Manual version setting (if no git available)
MANUAL_VERSION="0.9.0"  # <- Hier kannst du die Version manuell setzen

# Get version information
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "v${MANUAL_VERSION}")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "manual-build")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "Building CrowdNFO qBittorrent Post-Processor ${VERSION}"
echo "Git Commit: ${GIT_COMMIT}"
echo "Build Date: ${BUILD_DATE}"

# Build flags for version injection with static linking
LDFLAGS="-X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildDate=${BUILD_DATE} -extldflags '-static'"

echo "Building statically linked binaries for all platforms..."

# Build for Linux AMD64 (main binary)
echo "Building for Linux AMD64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags="${LDFLAGS}" -o crowdclient-qbittorrent-linux-amd64 .

# Build for Linux ARM64
echo "Building for Linux ARM64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -ldflags="${LDFLAGS}" -o crowdclient-qbittorrent-linux-arm64 .

# Build for macOS AMD64
echo "Building for macOS AMD64..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -ldflags="${LDFLAGS}" -o crowdclient-qbittorrent-darwin-amd64 .

# Build for macOS ARM64 (Apple Silicon)
echo "Building for macOS ARM64..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -a -ldflags="${LDFLAGS}" -o crowdclient-qbittorrent-darwin-arm64 .

# Build for Windows AMD64
echo "Building for Windows AMD64..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -a -ldflags="${LDFLAGS}" -o crowdclient-qbittorrent-windows-amd64.exe .

echo ""
echo "✅ Build completed successfully!"
