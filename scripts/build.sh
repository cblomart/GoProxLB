#!/bin/bash

# GoProxLB Build Script
# Builds static binaries for Linux AMD64, ARM, and ARM64

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Version (can be overridden with VERSION env var)
VERSION=${VERSION:-"dev"}
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

echo -e "${GREEN}Building GoProxLB v${VERSION}${NC}"
echo -e "${YELLOW}Build time: ${BUILD_TIME}${NC}"

# Create build directory
mkdir -p build

# Build flags
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# Build for Linux AMD64
echo -e "${YELLOW}Building for Linux AMD64...${NC}"
GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o build/goproxlb-linux-amd64 cmd/main.go

# Build for Linux ARM
echo -e "${YELLOW}Building for Linux ARM...${NC}"
GOOS=linux GOARCH=arm go build -ldflags="${LDFLAGS}" -o build/goproxlb-linux-arm cmd/main.go

# Build for Linux ARM64
echo -e "${YELLOW}Building for Linux ARM64...${NC}"
GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o build/goproxlb-linux-arm64 cmd/main.go

# Strip binaries (if strip command is available)
if command -v strip >/dev/null 2>&1; then
    echo -e "${YELLOW}Stripping binaries...${NC}"
    strip build/goproxlb-linux-amd64
    strip build/goproxlb-linux-arm
    strip build/goproxlb-linux-arm64
else
    echo -e "${YELLOW}strip command not found, skipping binary stripping${NC}"
fi

# Compress binaries with UPX (if available)
if command -v upx >/dev/null 2>&1; then
    echo -e "${YELLOW}Compressing binaries with UPX...${NC}"
    upx --best --lzma build/goproxlb-linux-amd64
    upx --best --lzma build/goproxlb-linux-arm
    upx --best --lzma build/goproxlb-linux-arm64
else
    echo -e "${YELLOW}UPX not found, skipping compression. Install with: brew install upx (macOS) or apt install upx (Ubuntu)${NC}"
fi

# Generate checksums
echo -e "${YELLOW}Generating SHA256 checksums...${NC}"
cd build
sha256sum goproxlb-linux-amd64 > goproxlb-linux-amd64.sha256
sha256sum goproxlb-linux-arm > goproxlb-linux-arm.sha256
sha256sum goproxlb-linux-arm64 > goproxlb-linux-arm64.sha256
cat *.sha256 > checksums.txt
cd ..

# Show file sizes
echo -e "${GREEN}Build completed successfully!${NC}"
echo -e "${YELLOW}Binary sizes:${NC}"
ls -lh build/goproxlb-linux-*

echo -e "${GREEN}Checksums:${NC}"
cat build/checksums.txt
