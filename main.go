package main

import (
	"bytes"
	"log"
	"os/exec"
	"time"
)

// Config
const (
	SyncInterval = 500 * time.Millisecond
	MaxTextSize  = 1024 * 1024 // 1MB limit to prevent crashes
)

func main() {
	log.Println("Starting Advanced VMware <-> Sway Clipboard Bridge...")
	log.Println("Files work? Good. This tool fixes the TEXT mismatch.")

	var lastWayland []byte
	var lastX11 []byte

	for {
		// 1. READ WAYLAND (Sway)
		// We use wl-paste to get what you just copied in Sway
		currentWayland, _ := exec.Command("wl-paste", "--no-newline").Output()

		// 2. READ X11 (VMware)
		// VMware puts Host text into X11 "CLIPBOARD". We try multiple targets.
		currentX11 := getX11Clipboard()

		// ---------------------------------------------------------
		// SYNC LOGIC
		// ---------------------------------------------------------

		// Case A: Wayland Changed (You copied in Sway) -> Push to X11 (VMware)
		if len(currentWayland) > 0 && !bytes.Equal(currentWayland, lastWayland) {
			// Only sync if it's different from what X11 already has
			if !bytes.Equal(currentWayland, currentX11) {
				log.Printf(">> Sway -> VMware: %d bytes", len(currentWayland))
				setX11Clipboard(currentWayland)
				// Update local cache so we don't loop
				lastX11 = currentWayland
				currentX11 = currentWayland
			}
			lastWayland = currentWayland
		}

		// Case B: X11 Changed (You copied in Windows Host) -> Push to Wayland
		if len(currentX11) > 0 && !bytes.Equal(currentX11, lastX11) {
			// Only sync if it's different from what Wayland already has
			if !bytes.Equal(currentX11, currentWayland) {
				log.Printf("<< VMware -> Sway: %d bytes", len(currentX11))
				setWaylandClipboard(currentX11)
				// Update local cache
				lastWayland = currentX11
				currentWayland = currentX11
			}
			lastX11 = currentX11
		}

		time.Sleep(SyncInterval)
	}
}

// getX11Clipboard tries robustly to get text from X11 in UTF-8
func getX11Clipboard() []byte {
	// Try UTF8_STRING first (Modern Linux/VMware)
	out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "UTF8_STRING", "-o").Output()
	if err == nil && len(out) > 0 {
		return out
	}

	// Fallback to STRING (Legacy)
	out, err = exec.Command("xclip", "-selection", "clipboard", "-t", "STRING", "-o").Output()
	if err == nil && len(out) > 0 {
		return out
	}

	return []byte{}
}

// setX11Clipboard pushes data to X11 for VMware to pick up
func setX11Clipboard(data []byte) {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-i")
	cmd.Stdin = bytes.NewReader(data)
	cmd.Run()
}

// setWaylandClipboard pushes data to Sway
func setWaylandClipboard(data []byte) {
	cmd := exec.Command("wl-copy")
	cmd.Stdin = bytes.NewReader(data)
	cmd.Run()
}
