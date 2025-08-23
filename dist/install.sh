#!/bin/bash
# Hype cross-platform installer
# Detects your OS and installs the appropriate binary

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Hype Cross-Platform Installer${NC}"
echo "=================================="

# Detect OS and architecture
OS=$(uname -s)
ARCH=$(uname -m)

case "$OS" in
    Linux*)
        if [[ "$ARCH" == "x86_64" ]]; then
            ARCHIVE="hype-v1.10.0-linux-amd64.tar.gz"
            echo -e "Detected: ${GREEN}Linux x86_64${NC}"
        elif [[ "$ARCH" == "aarch64" || "$ARCH" == "arm64" ]]; then
            ARCHIVE="hype-v1.10.0-linux-arm64.tar.gz"
            echo -e "Detected: ${GREEN}Linux ARM64${NC}"
        elif [[ "$ARCH" == "armv7l" || "$ARCH" == "armv6l" || "$ARCH" == "arm" ]]; then
            ARCHIVE="hype-v1.10.0-linux-arm.tar.gz"
            echo -e "Detected: ${GREEN}Linux ARM (32-bit)${NC}"
        else
            echo -e "${RED}Error: Unsupported Linux architecture: $ARCH${NC}"
            exit 1
        fi
        ;;
    Darwin*)
        if [[ "$ARCH" == "arm64" ]]; then
            ARCHIVE="hype-v1.10.0-darwin-arm64.tar.gz"
            echo -e "Detected: ${GREEN}macOS Apple Silicon (M1/M2)${NC}"
        elif [[ "$ARCH" == "x86_64" ]]; then
            ARCHIVE="hype-v1.10.0-darwin-amd64.tar.gz"
            echo -e "Detected: ${GREEN}macOS Intel${NC}"
        else
            echo -e "${RED}Error: Unsupported macOS architecture: $ARCH${NC}"
            exit 1
        fi
        ;;
    CYGWIN*|MINGW*|MSYS*)
        if [[ "$ARCH" == "x86_64" ]]; then
            ARCHIVE="hype-v1.10.0-windows-amd64.zip"
            echo -e "Detected: ${GREEN}Windows x86_64${NC}"
        else
            echo -e "${RED}Error: Unsupported Windows architecture: $ARCH${NC}"
            exit 1
        fi
        ;;
    *)
        echo -e "${RED}Error: Unsupported operating system: $OS${NC}"
        echo "Supported platforms: Linux, macOS, Windows"
        exit 1
        ;;
esac

# Create install directory
INSTALL_DIR="$HOME/.local/bin"
mkdir -p "$INSTALL_DIR"

# Download latest version
VERSION="v1.10.0"
URL="https://github.com/twilson63/hype/releases/download/$VERSION/$ARCHIVE"

echo -e "\n${YELLOW}Downloading Hype $VERSION...${NC}"
if command -v curl >/dev/null 2>&1; then
    curl -L -o "/tmp/$ARCHIVE" "$URL"
elif command -v wget >/dev/null 2>&1; then
    wget -O "/tmp/$ARCHIVE" "$URL"
else
    echo -e "${RED}Error: curl or wget is required for installation${NC}"
    exit 1
fi

echo -e "${YELLOW}Extracting archive...${NC}"
cd /tmp

# Extract based on file type
if [[ "$ARCHIVE" == *.zip ]]; then
    unzip -q "$ARCHIVE"
    EXTRACTED_DIR=$(unzip -l "$ARCHIVE" | awk 'NR==4 {print $4}' | cut -f1 -d"/")
else
    tar -xzf "$ARCHIVE"
    EXTRACTED_DIR=$(tar -tzf "$ARCHIVE" | head -1 | cut -f1 -d"/")
fi

# Find the binary
if [[ "$OS" == CYGWIN* || "$OS" == MINGW* || "$OS" == MSYS* ]]; then
    BINARY_PATH="/tmp/$EXTRACTED_DIR/hype.exe"
else
    BINARY_PATH="/tmp/$EXTRACTED_DIR/hype"
fi

# Make executable and install
chmod +x "$BINARY_PATH"
if [[ "$OS" == CYGWIN* || "$OS" == MINGW* || "$OS" == MSYS* ]]; then
    mv "$BINARY_PATH" "$INSTALL_DIR/hype.exe"
else
    mv "$BINARY_PATH" "$INSTALL_DIR/hype"
fi

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
    if [[ "$OS" == "Darwin"* ]]; then
        # macOS
        if [[ -f "$HOME/.zshrc" ]]; then
            SHELL_PROFILE="$HOME/.zshrc"
        elif [[ -f "$HOME/.bash_profile" ]]; then
            SHELL_PROFILE="$HOME/.bash_profile"
        elif [[ -f "$HOME/.bashrc" ]]; then
            SHELL_PROFILE="$HOME/.bashrc"
        fi
    else
        # Linux and others
        if [[ -f "$HOME/.bashrc" ]]; then
            SHELL_PROFILE="$HOME/.bashrc"
        elif [[ -f "$HOME/.zshrc" ]]; then
            SHELL_PROFILE="$HOME/.zshrc"
        elif [[ -f "$HOME/.profile" ]]; then
            SHELL_PROFILE="$HOME/.profile"
        fi
    fi
    
    if [[ -n "$SHELL_PROFILE" ]]; then
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_PROFILE"
        echo -e "Added to ${BLUE}$SHELL_PROFILE${NC}"
        echo -e "${YELLOW}Please run: ${BLUE}source $SHELL_PROFILE${NC} or restart your terminal"
    else
        echo -e "${YELLOW}Please add ${BLUE}~/.local/bin${NC} to your PATH manually"
    fi
fi

# Platform-specific notes
if [[ "$OS" == "Darwin"* ]]; then
    echo -e "\n${YELLOW}📋 macOS Note:${NC}"
    echo "If macOS blocks the binary (Gatekeeper), run:"
    echo -e "${BLUE}  xattr -d com.apple.quarantine ~/.local/bin/hype${NC}"
fi

echo -e "\n${GREEN}🎉 Installation complete!${NC}"
echo -e "Try: ${BLUE}hype --help${NC}"
echo -e "\n${BLUE}📚 Documentation: ${NC}https://github.com/twilson63/hype"
echo -e "${BLUE}🧩 Examples: ${NC}https://github.com/twilson63/hype/tree/main/examples"