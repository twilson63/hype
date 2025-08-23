#!/bin/bash
# Hype installer for Linux
# Downloads and installs the appropriate binary for your Linux system

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Hype Installer for Linux${NC}"
echo "=============================="

# Detect architecture
ARCH=$(uname -m)
if [[ "$ARCH" == "x86_64" ]]; then
    ARCHIVE="hype-v1.11.1-linux-amd64.tar.gz"
    echo -e "Detected: ${GREEN}Linux x86_64${NC}"
elif [[ "$ARCH" == "aarch64" || "$ARCH" == "arm64" ]]; then
    ARCHIVE="hype-v1.11.1-linux-arm64.tar.gz"
    echo -e "Detected: ${GREEN}Linux ARM64${NC}"
elif [[ "$ARCH" == "armv7l" || "$ARCH" == "armv6l" || "$ARCH" == "arm" ]]; then
    ARCHIVE="hype-v1.11.1-linux-arm.tar.gz"
    echo -e "Detected: ${GREEN}Linux ARM (32-bit)${NC}"
else
    echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
    echo "Supported architectures: x86_64, aarch64/arm64, armv7l/armv6l/arm"
    exit 1
fi

# Create install directory
INSTALL_DIR="$HOME/.local/bin"
mkdir -p "$INSTALL_DIR"

# Download latest version
VERSION="v1.11.1"
URL="https://github.com/twilson63/hype/releases/download/$VERSION/$ARCHIVE"

echo -e "\n${YELLOW}Downloading Hype $VERSION...${NC}"
curl -L -o "/tmp/$ARCHIVE" "$URL"

echo -e "${YELLOW}Extracting archive...${NC}"
cd /tmp
tar -xzf "$ARCHIVE"

# Find the binary in the extracted directory
EXTRACTED_DIR=$(tar -tzf "$ARCHIVE" | head -1 | cut -f1 -d"/")
BINARY_PATH="/tmp/$EXTRACTED_DIR/hype"

# Make executable and install
chmod +x "$BINARY_PATH"
mv "$BINARY_PATH" "$INSTALL_DIR/hype"

# Clean up
rm -rf "/tmp/$ARCHIVE" "/tmp/$EXTRACTED_DIR"

echo -e "${GREEN}✅ Hype installed successfully!${NC}"

# Verify installation by checking version
echo -e "\n${BLUE}🔍 Verifying installation...${NC}"
if command -v hype >/dev/null 2>&1; then
    INSTALLED_VERSION=$(hype version 2>/dev/null | head -n1 | awk '{print $2}' || echo "unknown")
    if [[ "$INSTALLED_VERSION" == "$VERSION" ]] || [[ "$INSTALLED_VERSION" == "${VERSION#v}" ]]; then
        echo -e "${GREEN}✅ Version verification successful: $INSTALLED_VERSION${NC}"
    else
        echo -e "${YELLOW}⚠️  Version mismatch: expected $VERSION, got $INSTALLED_VERSION${NC}"
    fi
else
    echo -e "${YELLOW}⚠️  Hype not found in PATH${NC}"
fi

# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    echo -e "\n${YELLOW}⚠️  Adding ~/.local/bin to your PATH...${NC}"
    
    # Add to shell profile
    SHELL_PROFILE=""
    if [[ -f "$HOME/.bashrc" ]]; then
        SHELL_PROFILE="$HOME/.bashrc"
    elif [[ -f "$HOME/.zshrc" ]]; then
        SHELL_PROFILE="$HOME/.zshrc"
    elif [[ -f "$HOME/.profile" ]]; then
        SHELL_PROFILE="$HOME/.profile"
    fi
    
    if [[ -n "$SHELL_PROFILE" ]]; then
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_PROFILE"
        echo -e "Added to ${BLUE}$SHELL_PROFILE${NC}"
        echo -e "${YELLOW}Please run: ${BLUE}source $SHELL_PROFILE${NC} or restart your terminal"
    else
        echo -e "${YELLOW}Please add ${BLUE}~/.local/bin${NC} to your PATH manually"
    fi
fi

echo -e "\n${GREEN}🎉 Installation complete!${NC}"
echo -e "Try: ${BLUE}hype --help${NC}"
echo -e "\n${BLUE}📚 Documentation: ${NC}https://github.com/twilson63/hype"
echo -e "${BLUE}🧩 Examples: ${NC}https://github.com/twilson63/hype/tree/main/examples"