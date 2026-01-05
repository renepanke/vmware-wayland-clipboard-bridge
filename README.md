# VMware Wayland Clipboard Bridge

A robust clipboard synchronization daemon for VMware guests running Wayland. Fixes copy-paste between host and
guest when `vmware-user` tools fail on Wayland.

## Features

- [x] **Configurable Timeouts** - Prevent hangs with per-command timeouts
- [x] **Size Limits** - Skip clipboard content exceeding size limits
- [x] **SHA256 Deduplication** - Hash-based change detection prevents loops
- [x] **Systemd Integration** - Auto-start on login via systemd user service
- [x] **Logging** - File-based or stdout logging with timestamps
- [x] **Configuration File** - TOML-based config at `~/.config/vmware-wayland-clipboard-bridge/config.toml`
- [x] **Bidirectional Sync** - Sway ↔ VMware clipboard synchronization

## Requirements

- Go 1.18+ (for building)
- `wl-clipboard` (for Wayland clipboard access)
- `xclip` (for X11 clipboard access)
- `open-vm-tools` (VMware guest tools)

## Installation

### 1. Install Dependencies

E.g. on Fedora:

```bash
sudo dnf install golang wl-clipboard xclip open-vm-tools open-vm-tools-desktop
```

### 2. Prepare Directory

```bash
git clone https://github.com/renepanke/vmware-wayland-clipboard-bridge.git
```

### 3. Run Installer

```bash
chmod +x install.sh
./install.sh
```

The installer will:

- Build the Go binary
- Create config directory: `~/.config/vmware-wayland-clipboard-bridge/`
- Create log directory: `~/.local/share/vmware-wayland-clipboard-bridge/`
- Install binary to: `~/.local/bin/vmware-wayland-clipboard-bridge`
- Register systemd service

### 4. Enable & Start Service

```bash
# Enable autostart on login
systemctl --user enable vmware-wayland-clipboard-bridge.service

# Start the service now
systemctl --user start vmware-wayland-clipboard-bridge.service

# Check status
systemctl --user status vmware-wayland-clipboard-bridge.service

# View live logs
journalctl --user -u vmware-wayland-clipboard-bridge.service -f
```

## Configuration

Edit `~/.config/vmware-wayland-clipboard-bridge/config.toml`:

```toml
[timeouts]
command_timeout = 2                # Timeout per command (seconds)
max_clipboard_size = 52428800     # Max content size (50MB)

[sync]
interval_ms = 500                  # Check interval (milliseconds)
enable_logging = true              # Enable sync logs

[logging]
verbose = true                      # Print detailed logs
log_file = "~/.local/share/vmware-wayland-clipboard-bridge/sync.log"  # Log location
```

### Tuning Tips

**For better performance (lower CPU):**

```toml
[sync]
interval_ms = 1000  # Check every 1 second instead of 500ms
```

**For faster sync (more responsive):**

```toml
[sync]
interval_ms = 200   # Check every 200ms (may use more CPU)
```

**For larger clipboard content:**

```toml
[timeouts]
max_clipboard_size = 104857600  # 100MB instead of 50MB
command_timeout = 5              # Increase timeout to 5 seconds
```

**For file-based logging (production):**

```toml
[logging]
log_file = "~/.local/share/vmware-wayland-clipboard-bridge/sync.log"
```

**For stdout logging (debugging):**

```toml
[logging]
log_file = ""  # Empty = use stdout
```

## Usage

### Via Systemd (Recommended)

The service auto-starts on login. No additional action needed.

```bash
# View logs
journalctl --user -u vmware-wayland-clipboard-bridge.service -f

# Restart service
systemctl --user restart vmware-wayland-clipboard-bridge.service

# Stop service
systemctl --user stop vmware-wayland-clipboard-bridge.service
```

### Manual Testing

```bash
~/.local/bin/vmware-wayland-clipboard-bridge
```

Press `Ctrl+C` to stop.

## Troubleshooting

### Clipboard still not syncing

1. **Verify `vmware-user` is running:**
   ```bash
   pgrep -a vmware-user
   ```
   If not found, run:
   ```bash
   vmware-user-suid-wrapper
   ```

2. **Check if the sync daemon is active:**
   ```bash
   systemctl --user is-active vmware-wayland-clipboard-bridge.service
   ```

3. **View logs for errors:**
   ```bash
   journalctl --user -u vmware-wayland-clipboard-bridge.service -n 50
   ```

4. **Verify clipboard tools work manually:**
   ```bash
   # Test Wayland
   wl-paste
   echo "test" | wl-copy
   
   # Test X11
   xclip -selection clipboard -o
   echo "test" | xclip -selection clipboard -i
   ```

5. **Increase timeout if commands are slow:**
   Edit config.toml and set `command_timeout = 5`

### High CPU Usage

- Increase `interval_ms` in config.toml (e.g., 1000 instead of 500)
- Check if a clipboard tool is hung: `ps aux | grep -E 'wl-paste|wl-copy|xclip'`

### Can't start service

1. Check if all dependencies are installed
2. Verify paths exist:
   ```bash
   ls -la ~/.local/bin/vmware-wayland-clipboard-bridge
   ls -la ~/.config/vmware-wayland-clipboard-bridge/config.toml
   ```
3. Check for errors:
   ```bash
   ~/.local/bin/vmware-wayland-clipboard-bridge
   ```

### Uninstall

```bash
cd ~/vmware-wayland-clipboard-bridge
./uninstall.sh
```

This removes the service and binary but keeps config files. To fully remove:

```bash
rm -rf ~/.config/vmware-wayland-clipboard-bridge/
rm -rf ~/.local/share/vmware-wayland-clipboard-bridge/
```

## How It Works

1. **Polls** both Wayland and X11 clipboards at configurable interval
2. **Compares** SHA256 hashes to detect changes (prevents loops)
3. **Syncs** if one side changed and differs from the other
4. **Enforces** timeout limits to prevent hangs
5. **Respects** size limits to prevent memory issues
6. **Logs** all operations for debugging

## Why This Works

VMware tools on Linux rely on the X11 clipboard, but state of the art is Wayland. This daemon bridges the gap by:

- Reading from Wayland clipboard (via `wl-paste`)
- Writing to VMware's X11 clipboard (via `xclip`)
- Detecting changes and syncing bidirectionally
- Handling edge cases like timeouts and size limits