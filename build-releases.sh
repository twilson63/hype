#!/bin/bash

# Build script for creating Hype releases for multiple platforms
# This script builds binaries for different platforms and packages them for distribution

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

# Handle command line arguments first
case "${1:-}" in
    --help|-h)
        echo "Hype Release Builder"
        echo "Usage: ./build-releases.sh <version> [--clean]"
        echo "       ./build-releases.sh --help"
        echo ""
        echo "Arguments:"
        echo "  <version>  Version to build (e.g., v1.11.1)"
        echo ""
        echo "Options:"
        echo "  --clean    Clean build directory only"
        echo "  --help     Show this help message"
        echo ""
        echo "Examples:"
        echo "  ./build-releases.sh v1.11.1"
        echo "  ./build-releases.sh --clean"
        exit 0
        ;;
    --clean)
        echo -e "${BLUE}[INFO]${NC} Cleaning build directory only..."
        rm -rf "dist"
        echo -e "${GREEN}[SUCCESS]${NC} Build directory cleaned"
        exit 0
        ;;
esac

# Configuration
# Use provided version or try to detect from git tags
if [[ -n "${1:-}" ]]; then
    VERSION="$1"
    echo "Using provided version: $VERSION"
else
    # If we're exactly on a tag, use just the tag
    if git describe --exact-match --tags HEAD 2>/dev/null; then
        VERSION=$(git describe --exact-match --tags HEAD)
        echo "Using current git tag: $VERSION"
    else
        echo "Error: No version specified and current commit is not tagged."
        echo "Usage: $0 <version> [--clean]"
        echo "Example: $0 v1.11.1"
        exit 1
    fi
fi
BUILD_DIR="dist"
BINARY_NAME="hype"

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Platforms to build for
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "linux/arm"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

clean_build_dir() {
    log_info "Cleaning build directory..."
    rm -rf "$BUILD_DIR"
    mkdir -p "$BUILD_DIR"
}

build_binary() {
    local goos="$1"
    local goarch="$2"
    local output_name="$3"
    
    log_info "Building for $goos/$goarch..."
    
    # Get build information
    local commit=$(git rev-parse --short HEAD)
    local date=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    # Build with version information
    env GOOS="$goos" GOARCH="$goarch" go build -ldflags "
        -s -w 
        -X main.version=$VERSION 
        -X main.commit=$commit 
        -X main.date=$date
    " -o "$output_name" .
    
    if [[ $? -eq 0 ]]; then
        log_success "Built $output_name"
    else
        log_error "Failed to build for $goos/$goarch"
        return 1
    fi
}

package_release() {
    local platform="$1"
    local binary_path="$2"
    
    local platform_dir="$BUILD_DIR/$platform"
    mkdir -p "$platform_dir"
    
    # Copy binary
    cp "$binary_path" "$platform_dir/"
    
    # Copy documentation and examples
    cp README.md "$platform_dir/"
    cp -r examples "$platform_dir/"
    
    # Create archive
    cd "$BUILD_DIR"
    
    if [[ "$platform" == *"windows"* ]]; then
        # Create ZIP for Windows
        zip -r "hype-${VERSION}-${platform}.zip" "$platform"
        log_success "Created hype-${VERSION}-${platform}.zip"
    else
        # Create tar.gz for Unix systems
        tar -czf "hype-${VERSION}-${platform}.tar.gz" "$platform"
        log_success "Created hype-${VERSION}-${platform}.tar.gz"
    fi
    
    cd ..
}

generate_checksums() {
    log_info "Generating checksums..."
    
    cd "$BUILD_DIR"
    
    # Generate SHA256 checksums
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum *.tar.gz *.zip > "hype-${VERSION}-checksums.txt" 2>/dev/null || true
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 *.tar.gz *.zip > "hype-${VERSION}-checksums.txt" 2>/dev/null || true
    fi
    
    cd ..
    
    if [[ -f "$BUILD_DIR/hype-${VERSION}-checksums.txt" ]]; then
        log_success "Generated checksums file"
    fi
}

create_arweave_manifest() {
    log_info "Creating Arweave deployment manifest..."
    
    cat > "$BUILD_DIR/arweave-manifest.json" << EOF
{
  "manifest": "arweave/paths",
  "version": "0.1.0",
  "index": {
    "path": "install.sh"
  },
  "paths": {
    "install.sh": {
      "id": "INSTALL_SCRIPT_TX_ID"
    }
EOF

    # Add each release file to the manifest
    cd "$BUILD_DIR"
    for file in *.tar.gz *.zip; do
        if [[ -f "$file" ]]; then
            cat >> "arweave-manifest.json" << EOF
    ,
    "$file": {
      "id": "REPLACE_WITH_TX_ID"
    }
EOF
        fi
    done
    
    cat >> "arweave-manifest.json" << EOF

  }
}
EOF
    
    cd ..
    log_success "Created Arweave manifest"
}

main() {
    echo "Building Hype releases for version $VERSION"
    echo
    
    # Clean build directory
    clean_build_dir
    
    # Build for each platform
    for platform in "${PLATFORMS[@]}"; do
        IFS='/' read -r goos goarch <<< "$platform"
        
        binary_name="${BINARY_NAME}"
        if [[ "$goos" == "windows" ]]; then
            binary_name="${BINARY_NAME}.exe"
        fi
        
        binary_path="$BUILD_DIR/$binary_name"
        
        # Build binary
        if build_binary "$goos" "$goarch" "$binary_path"; then
            # Package release
            package_release "$goos-$goarch" "$binary_path"
            
            # Clean up binary
            rm -f "$binary_path"
        fi
    done
    
    # Copy install script to build directory
    cp install.sh "$BUILD_DIR/"
    
    # Generate checksums
    generate_checksums
    
    # Create Arweave manifest
    create_arweave_manifest
    
    echo
    log_success "Build completed! Release files are in $BUILD_DIR/"
    echo
    echo "Files created:"
    ls -la "$BUILD_DIR/"
    echo
    echo "To deploy to Arweave:"
    echo "1. Upload each release file and the install.sh script to Arweave"
    echo "2. Update the install.sh script with the correct Arweave URLs"
    echo "3. Share the install script URL for one-command installation"
    echo
    echo "Installation command will be:"
    echo "curl -sSL https://arweave.net/YOUR_INSTALL_SCRIPT_TX_ID | bash"
}

# Skip the version argument when calling main
shift 2>/dev/null || true
main "$@"