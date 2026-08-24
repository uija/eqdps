//go:build !windows

package overlay

func handleNativeOverlayEvent(_ *Overlay, _ any) {}

func captureNativeOverlayPosition(_ *Overlay) {}

func setNativeOverlayOpacity(overlay *Overlay, opacity float32) {
}

func nativeOpacityAvailable() bool { return false }
