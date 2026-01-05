package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
)

// Config represents the configuration file structure
type Config struct {
	Timeouts Timeouts      `toml:"timeouts"`
	Sync     SyncConfig    `toml:"sync"`
	Logging  LoggingConfig `toml:"logging"`
}

type Timeouts struct {
	CommandTimeout   int `toml:"command_timeout"`    // seconds
	MaxClipboardSize int `toml:"max_clipboard_size"` // bytes
}

type SyncConfig struct {
	IntervalMs    int  `toml:"interval_ms"`
	EnableLogging bool `toml:"enable_logging"`
}

type LoggingConfig struct {
	Verbose bool   `toml:"verbose"`
	LogFile string `toml:"log_file"` // empty = stdout
}

// ClipboardManager handles syncing between Wayland and X11 clipboards
type ClipboardManager struct {
	lastWaylandHash  string
	lastX11Hash      string
	syncInterval     time.Duration
	commandTimeout   time.Duration
	maxClipboardSize int
	enableLogging    bool
	logWriter        io.WriteCloser
	logger           *log.Logger
}

// NewClipboardManager creates a new clipboard manager from config
func NewClipboardManager(config Config) (*ClipboardManager, error) {
	var logOutput io.WriteCloser = os.Stdout

	if config.Logging.LogFile != "" {
		// Expand ~ to home directory
		logPath := os.ExpandEnv(config.Logging.LogFile)
		if len(logPath) > 0 && logPath[0] == '~' {
			home, _ := os.UserHomeDir()
			logPath = filepath.Join(home, logPath[1:])
		}

		// Ensure directory exists
		logDir := filepath.Dir(logPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logOutput = f
	}

	logger := log.New(logOutput, "[vmware-wayland-clipboard-bridge] ", log.LstdFlags)

	return &ClipboardManager{
		syncInterval:     time.Duration(config.Sync.IntervalMs) * time.Millisecond,
		commandTimeout:   time.Duration(config.Timeouts.CommandTimeout) * time.Second,
		maxClipboardSize: config.Timeouts.MaxClipboardSize,
		enableLogging:    config.Logging.Verbose,
		logWriter:        logOutput,
		logger:           logger,
	}, nil
}

// hash returns a SHA256 hash of the content
func (cm *ClipboardManager) hash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// getWaylandClipboard reads from Wayland clipboard with timeout
func (cm *ClipboardManager) getWaylandClipboard() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cm.commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "wl-paste", "-t", "text/plain")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			cm.logf("warning: wl-paste timeout")
		}
		return "", nil // Return empty, not error (clipboard may be unavailable)
	}

	content := out.String()
	if len(content) > cm.maxClipboardSize {
		cm.logf("warning: wayland clipboard exceeds size limit (%d > %d bytes)", len(content), cm.maxClipboardSize)
		return "", nil
	}

	return content, nil
}

// setWaylandClipboard writes to Wayland clipboard with timeout
func (cm *ClipboardManager) setWaylandClipboard(content string) error {
	if len(content) > cm.maxClipboardSize {
		cm.logf("error: content exceeds max size, skipping wayland sync (%d > %d bytes)", len(content), cm.maxClipboardSize)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cm.commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "wl-copy", "-t", "text/plain")
	cmd.Stdin = bytes.NewBufferString(content)

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			cm.logf("warning: wl-copy timeout")
		} else {
			cm.logf("warning: failed to set wayland clipboard: %v", err)
		}
		return nil // Non-fatal error
	}

	return nil
}

// getX11Clipboard reads from X11 clipboard with timeout
func (cm *ClipboardManager) getX11Clipboard() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cm.commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-o")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			cm.logf("warning: xclip read timeout")
		}
		return "", nil // Return empty, not error (clipboard may be unavailable)
	}

	content := out.String()
	if len(content) > cm.maxClipboardSize {
		cm.logf("warning: x11 clipboard exceeds size limit (%d > %d bytes)", len(content), cm.maxClipboardSize)
		return "", nil
	}

	return content, nil
}

// setX11Clipboard writes to X11 clipboard with timeout
func (cm *ClipboardManager) setX11Clipboard(content string) error {
	if len(content) > cm.maxClipboardSize {
		cm.logf("error: content exceeds max size, skipping x11 sync (%d > %d bytes)", len(content), cm.maxClipboardSize)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cm.commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-i")
	cmd.Stdin = bytes.NewBufferString(content)

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			cm.logf("warning: xclip write timeout")
		} else {
			cm.logf("warning: failed to set x11 clipboard: %v", err)
		}
		return nil
	}
	return nil
}

// syncClipboards performs one sync cycle: Wayland <-> X11
func (cm *ClipboardManager) syncClipboards() error {
	// Read from both clipboards
	waylandContent, _ := cm.getWaylandClipboard()
	x11Content, _ := cm.getX11Clipboard()

	waylandHash := cm.hash(waylandContent)
	x11Hash := cm.hash(x11Content)

	// Sync Wayland -> X11 (if Wayland changed)
	if waylandHash != cm.lastWaylandHash && waylandContent != "" {
		if waylandHash != x11Hash { // Only sync if they differ
			cm.logf(">> wayland -> vmware: %d bytes", len(waylandContent))
			if err := cm.setX11Clipboard(waylandContent); err != nil {
				cm.logf("error syncing to x11: %v", err)
			}
			cm.lastX11Hash = waylandHash
			x11Hash = waylandHash
		}
		cm.lastWaylandHash = waylandHash
	}

	// Sync X11 -> Wayland (if X11 changed)
	if x11Hash != cm.lastX11Hash && x11Content != "" {
		if x11Hash != waylandHash { // Only sync if they differ
			cm.logf("<< vmware -> wayland: %d bytes", len(x11Content))
			if err := cm.setWaylandClipboard(x11Content); err != nil {
				cm.logf("error syncing to wayland: %v", err)
			}
			cm.lastWaylandHash = x11Hash
			waylandHash = x11Hash
		}
		cm.lastX11Hash = x11Hash
	}

	return nil
}

// start begins the continuous sync loop
func (cm *ClipboardManager) start() {
	ticker := time.NewTicker(cm.syncInterval)
	defer ticker.Stop()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	cm.logf("clipboard sync started (interval: %v, timeout: %v, max size: %d bytes)",
		cm.syncInterval, cm.commandTimeout, cm.maxClipboardSize)

	for {
		select {
		case <-ticker.C:
			if err := cm.syncClipboards(); err != nil {
				cm.logf("sync error: %v", err)
			}
		case <-sigChan:
			cm.logf("clipboard sync stopped")
			if cm.logWriter != os.Stdout {
				if err := cm.logWriter.Close(); err != nil {
					cm.logf("warning: error closing log file: %v", err)
				}
			}
			return
		}
	}
}

// logf logs a formatted message if verbose is enabled
func (cm *ClipboardManager) logf(format string, v ...interface{}) {
	if cm.enableLogging {
		cm.logger.Printf(format, v...)
	}
}

// LoadConfig loads the configuration from file or returns defaults
func LoadConfig() (Config, error) {
	// Try multiple config paths
	home, _ := os.UserHomeDir()
	configPaths := []string{
		filepath.Join(home, ".config/vmware-sway-sync/config.toml"),
		filepath.Join(home, ".vmware-sway-sync.toml"),
		"/etc/vmware-sway-sync/config.toml",
	}

	var config Config
	var configPath string

	// Look for existing config
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			configPath = path
			break
		}
	}

	if configPath != "" {
		_, err := toml.DecodeFile(configPath, &config)
		if err != nil {
			return Config{}, fmt.Errorf("failed to parse config: %w", err)
		}
	} else {
		// Use defaults if no config file found
		config = getDefaultConfig()
	}

	return config, nil
}

func getDefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Timeouts: Timeouts{
			CommandTimeout:   2,
			MaxClipboardSize: 52428800, // 50MB
		},
		Sync: SyncConfig{
			IntervalMs:    500,
			EnableLogging: true,
		},
		Logging: LoggingConfig{
			Verbose: true,
			LogFile: filepath.Join(home, ".local/share/vmware-sway-sync/sync.log"),
		},
	}
}

func main() {
	tools := []string{"wl-paste", "wl-copy", "xclip"}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: %q not found. install it with:\n", tool)
			_, _ = fmt.Fprintf(os.Stderr, "  sudo dnf install wl-clipboard xclip\n")
			os.Exit(1)
		}
	}

	config, err := LoadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		_, _ = fmt.Fprintf(os.Stderr, "using defaults...\n")
		config = getDefaultConfig()
	}

	manager, err := NewClipboardManager(config)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	manager.start()
}
