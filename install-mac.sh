#!/bin/bash
# Hype installer for macOS
# Downloads and installs the appropriate binary for your Mac

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Hype Installer for macOS${NC}"
echo "=============================="

# Detect architecture
ARCH=$(uname -m)
if [[ "$ARCH" == "arm64" ]]; then
    ARCHIVE="hype-v1.10.0-darwin-arm64.tar.gz"
    echo -e "Detected: ${GREEN}Apple Silicon (M1/M2)${NC}"
elif [[ "$ARCH" == "x86_64" ]]; then
    ARCHIVE="hype-v1.10.0-darwin-amd64.tar.gz"
    echo -e "Detected: ${GREEN}Intel Mac${NC}"
else
    echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
    exit 1
fi

# Create install directory
INSTALL_DIR="$HOME/.local/bin"
mkdir -p "$INSTALL_DIR"

# Download latest version
VERSION="v1.11.0"
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
    if [[ -f "$HOME/.zshrc" ]]; then
        SHELL_PROFILE="$HOME/.zshrc"
    elif [[ -f "$HOME/.bash_profile" ]]; then
        SHELL_PROFILE="$HOME/.bash_profile"
    elif [[ -f "$HOME/.bashrc" ]]; then
        SHELL_PROFILE="$HOME/.bashrc"
    fi
    
    if [[ -n "$SHELL_PROFILE" ]]; then
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_PROFILE"
        echo -e "Added to ${BLUE}$SHELL_PROFILE${NC}"
        echo -e "${YELLOW}Please run: ${BLUE}source $SHELL_PROFILE${NC} or restart your terminal"
    else
        echo -e "${YELLOW}Please add ${BLUE}~/.local/bin${NC} to your PATH manually"
    fi
fi

# macOS Gatekeeper notice
echo -e "\n${YELLOW}📋 First Run Instructions:${NC}"
echo "If macOS blocks the binary (Gatekeeper), run:"
echo -e "${BLUE}  xattr -d com.apple.quarantine ~/.local/bin/hype${NC}"
echo "Then try running hype again."

echo -e "\n${GREEN}🎉 Installation complete!${NC}"
echo -e "Try: ${BLUE}hype --help${NC}"