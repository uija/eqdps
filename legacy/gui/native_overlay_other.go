//go:build !windows

package main

func handleNativeOverlayEvent(_ *combatOverlay, _ any) {}

func captureNativeOverlayPosition(_ *combatOverlay) {}

func setNativeOverlayOpacity(overlay *combatOverlay, opacity float32) {
	overlay.nativeMu.Lock()
	overlay.nativeOpacity = opacity
	overlay.nativeMu.Unlock()
}

func nativeOpacityAvailable() bool { return false }
