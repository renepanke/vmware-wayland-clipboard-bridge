#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== VMware Wayland Clipboard Bridge Installer ===${NC}"

# Check Go installation
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed${NC}"
    echo "Install golang with your package manager"
    exit 1
fi

# Check required tools
for tool in wl-paste wl-copy xclip; do
    if ! command -v $tool &> /dev/null; then
        echo -e "${RED}Error: $tool is not installed${NC}"
        exit 1
    fi
done

# Create config directory
CONFIG_DIR="$HOME/.config/vmware-wayland-clipboard-bridge"
mkdir -p "$CONFIG_DIR"

# Create log directory
LOG_DIR="$HOME/.local/share/vmware-wayland-clipboard-bridge"
mkdir -p "$LOG_DIR"

# Check if config file exists, create if not
if [ ! -f "$CONFIG_DIR/config.toml" ]; then
    echo -e "${BLUE}Creating config file...${NC}"
    cp config.toml "$CONFIG_DIR/config.toml"
    echo -e "${GREEN}Config created at: $CONFIG_DIR/config.toml${NC}"
else
    echo "Config file already exists at: $CONFIG_DIR/config.toml"
fi

# Build the program
echo -e "${BLUE}Building vmware-wayland-clipboard-bridge...${NC}"
go build -o vmware-wayland-clipboard-bridge main.go

if [ ! -f vmware-wayland-clipboard-bridge ]; then
    echo -e "${RED}Build failed${NC}"
    exit 1
fi

echo -e "${GREEN}Build successful${NC}"

# Create bin directory
BIN_DIR="$HOME/.local/bin"
mkdir -p "$BIN_DIR"

# Install binary
echo -e "${BLUE}Installing binary to $BIN_DIR...${NC}"
cp vmware-wayland-clipboard-bridge "$BIN_DIR/"
chmod +x "$BIN_DIR/vmware-wayland-clipboard-bridge"

# Install systemd service
SERVICE_DIR="$HOME/.config/systemd/user"
mkdir -p "$SERVICE_DIR"
echo -e "${BLUE}Installing systemd service...${NC}"
cp vmware-wayland-clipboard-bridge.service "$SERVICE_DIR/"

# Reload systemd
systemctl --user daemon-reload

echo ""
echo -e "${GREEN}=== Installation Complete ===${NC}"
echo ""
echo "Next steps:"
echo "1. Review configuration: $CONFIG_DIR/config.toml"
echo "2. Enable autostart:"
echo "   systemctl --user enable vmware-wayland-clipboard-bridge.service"
echo "3. Start the service:"
echo "   systemctl --user start vmware-wayland-clipboard-bridge.service"
echo "4. Check status:"
echo "   systemctl --user status vmware-wayland-clipboard-bridge.service"
echo "5. View logs:"
echo "   journalctl --user -u vmware-wayland-clipboard-bridge.service -f"
echo ""
echo "Or run manually:"
echo "   $BIN_DIR/vmware-wayland-clipboard-bridge"
