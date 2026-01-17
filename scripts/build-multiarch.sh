#!/bin/bash
# Multi-architecture build script for Cold Storage Management System
# Builds for both Mac Mini M4 (darwin/arm64) and Linux (linux/amd64)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/build"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

echo -e "${GREEN}=== Cold Storage Multi-Architecture Build ===${NC}"
echo "Version: ${VERSION}"
echo "Build Time: ${BUILD_TIME}"
echo ""

# Clean build directory
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

cd "${PROJECT_ROOT}"

# Build function
build_target() {
    local os=$1
    local arch=$2
    local output_name=$3

    echo -e "${YELLOW}Building for ${os}/${arch}...${NC}"

    local output_path="${BUILD_DIR}/${output_name}"

    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build \
        -ldflags "${LDFLAGS}" \
        -o "${output_path}" \
        ./cmd/server

    if [ -f "${output_path}" ]; then
        local size=$(ls -lh "${output_path}" | awk '{print $5}')
        echo -e "${GREEN}  Built: ${output_path} (${size})${NC}"
    else
        echo -e "${RED}  Failed to build ${output_path}${NC}"
        return 1
    fi
}

# Build for Mac Mini M4 (Primary server)
echo -e "\n${GREEN}=== Building for Mac Mini M4 (Primary) ===${NC}"
build_target "darwin" "arm64" "server-darwin-arm64"

# Build for Linux (Secondary/Archive server)
echo -e "\n${GREEN}=== Building for Linux Server (Secondary) ===${NC}"
build_target "linux" "amd64" "server-linux-amd64"

# Copy static files and templates
echo -e "\n${YELLOW}Copying static files and templates...${NC}"

# Create distribution packages
echo -e "\n${GREEN}=== Creating Distribution Packages ===${NC}"

# Mac Mini package
MAC_PKG_DIR="${BUILD_DIR}/coldstore-mac-${VERSION}"
mkdir -p "${MAC_PKG_DIR}"
cp "${BUILD_DIR}/server-darwin-arm64" "${MAC_PKG_DIR}/server"
cp -r "${PROJECT_ROOT}/templates" "${MAC_PKG_DIR}/" 2>/dev/null || true
cp -r "${PROJECT_ROOT}/static" "${MAC_PKG_DIR}/" 2>/dev/null || true
cp -r "${PROJECT_ROOT}/configs" "${MAC_PKG_DIR}/" 2>/dev/null || true
cp "${PROJECT_ROOT}/deploy/macos/com.coldstore.server.plist" "${MAC_PKG_DIR}/" 2>/dev/null || true
cp "${PROJECT_ROOT}/docs/MAC_MINI_SETUP.md" "${MAC_PKG_DIR}/SETUP.md" 2>/dev/null || true

# Create tarball for Mac
(cd "${BUILD_DIR}" && tar -czf "coldstore-mac-${VERSION}.tar.gz" "coldstore-mac-${VERSION}")
echo -e "${GREEN}  Created: build/coldstore-mac-${VERSION}.tar.gz${NC}"

# Linux package
LINUX_PKG_DIR="${BUILD_DIR}/coldstore-linux-${VERSION}"
mkdir -p "${LINUX_PKG_DIR}"
cp "${BUILD_DIR}/server-linux-amd64" "${LINUX_PKG_DIR}/server"
cp -r "${PROJECT_ROOT}/templates" "${LINUX_PKG_DIR}/" 2>/dev/null || true
cp -r "${PROJECT_ROOT}/static" "${LINUX_PKG_DIR}/" 2>/dev/null || true
cp -r "${PROJECT_ROOT}/configs" "${LINUX_PKG_DIR}/" 2>/dev/null || true
cp "${PROJECT_ROOT}/deploy/linux/coldstore.service" "${LINUX_PKG_DIR}/" 2>/dev/null || true
cp "${PROJECT_ROOT}/docs/PRODUCTION_SETUP_GUIDE.md" "${LINUX_PKG_DIR}/SETUP.md" 2>/dev/null || true

# Create tarball for Linux
(cd "${BUILD_DIR}" && tar -czf "coldstore-linux-${VERSION}.tar.gz" "coldstore-linux-${VERSION}")
echo -e "${GREEN}  Created: build/coldstore-linux-${VERSION}.tar.gz${NC}"

# Clean up temp directories
rm -rf "${MAC_PKG_DIR}" "${LINUX_PKG_DIR}"

# Summary
echo -e "\n${GREEN}=== Build Summary ===${NC}"
echo "Version: ${VERSION}"
echo ""
echo "Artifacts:"
ls -lh "${BUILD_DIR}"/*.tar.gz 2>/dev/null || true
ls -lh "${BUILD_DIR}"/server-* 2>/dev/null || true

echo ""
echo -e "${GREEN}Build completed successfully!${NC}"
echo ""
echo "To deploy:"
echo "  Mac Mini (192.168.15.240):"
echo "    scp build/coldstore-mac-${VERSION}.tar.gz admin@192.168.15.240:/tmp/"
echo "    ssh admin@192.168.15.240 'cd /tmp && tar xzf coldstore-mac-${VERSION}.tar.gz'"
echo ""
echo "  Linux Server (192.168.15.241):"
echo "    scp build/coldstore-linux-${VERSION}.tar.gz admin@192.168.15.241:/tmp/"
echo "    ssh admin@192.168.15.241 'cd /tmp && tar xzf coldstore-linux-${VERSION}.tar.gz'"
