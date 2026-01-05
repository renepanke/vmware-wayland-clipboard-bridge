#!/bin/bash
# Quick uninstall script for vmware-wayland-clipboard-bridge

echo "Uninstalling vmware-wayland-clipboard-bridge..."

# Stop and disable service
echo "Stopping systemd service..."
systemctl --user stop vmware-wayland-clipboard-bridge.service 2>/dev/null || true
systemctl --user disable vmware-wayland-clipboard-bridge.service 2>/dev/null || true

# Remove service file
SERVICE_DIR="$HOME/.config/systemd/user"
rm -f "$SERVICE_DIR/vmware-wayland-clipboard-bridge.service"

# Remove binary
BIN_DIR="$HOME/.local/bin"
rm -f "$BIN_DIR/vmware-wayland-clipboard-bridge"

# Reload systemd
systemctl --user daemon-reload

echo "Done. Config files and logs remain at:"
echo "  ~/.config/vmware-wayland-clipboard-bridge/"
echo "  ~/.local/share/vmware-wayland-clipboard-bridge/"
echo ""
echo "To remove them manually:"
echo "  rm -rf ~/.config/vmware-wayland-clipboard-bridge/"
echo "  rm -rf ~/.local/share/vmware-wayland-clipboard-bridge/"
