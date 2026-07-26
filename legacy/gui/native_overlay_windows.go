//go:build windows

package main

import (
	"time"
	"unsafe"

	"gioui.org/app"
	"golang.org/x/sys/windows"
)

const (
	gwlpExStyle   = -20
	wsExLayered   = 0x00080000
	lwaAlpha      = 0x00000002
	swpNoSize     = 0x0001
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

var (
	user32                         = windows.NewLazySystemDLL("user32.dll")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procGetWindowLong              = user32.NewProc("GetWindowLongW")
	procSetWindowLong              = user32.NewProc("SetWindowLongW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
)

type winRect struct {
	left   int32
	top    int32
	right  int32
	bottom int32
}

func handleNativeOverlayEvent(overlay *combatOverlay, event any) {
	view, ok := event.(app.Win32ViewEvent)
	if !ok || view.HWND == 0 {
		return
	}
	overlay.nativeMu.Lock()
	overlay.nativeHandle = view.HWND
	restorePosition := overlay.hasSavedPosition && !overlay.positionRestored
	if restorePosition {
		overlay.positionRestored = true
	}
	x, y := overlay.savedX, overlay.savedY
	overlay.nativeMu.Unlock()
	go func(handle uintptr) {
		time.Sleep(100 * time.Millisecond)
		if restorePosition {
			moveOverlayToSavedPosition(windows.Handle(handle), x, y)
		}
		applyDesiredNativeOverlayOpacity(overlay, handle)
	}(view.HWND)
}

func captureNativeOverlayPosition(overlay *combatOverlay) {
	overlay.nativeMu.Lock()
	defer overlay.nativeMu.Unlock()
	if overlay.nativeHandle == 0 {
		return
	}
	hwnd := windows.Handle(overlay.nativeHandle)
	if rect, ok := windowRect(hwnd); ok {
		overlay.lastX = int(rect.left)
		overlay.lastY = int(rect.top)
		overlay.positionKnown = true
	}
}

func setNativeOverlayOpacity(overlay *combatOverlay, opacity float32) {
	overlay.nativeMu.Lock()
	defer overlay.nativeMu.Unlock()
	overlay.nativeOpacity = opacity
	if overlay.nativeHandle != 0 {
		applyWindowOpacity(windows.Handle(overlay.nativeHandle), opacity)
	}
}

func applyDesiredNativeOverlayOpacity(overlay *combatOverlay, handle uintptr) {
	overlay.nativeMu.Lock()
	defer overlay.nativeMu.Unlock()
	if overlay.nativeHandle == handle {
		applyWindowOpacity(windows.Handle(handle), overlay.nativeOpacity)
	}
}

func applyWindowOpacity(hwnd windows.Handle, opacity float32) {
	opacity = max(0, min(1, opacity))
	index := int32(gwlpExStyle)
	style, _, _ := procGetWindowLong.Call(uintptr(hwnd), uintptr(index))
	if style&wsExLayered == 0 {
		procSetWindowLong.Call(uintptr(hwnd), uintptr(index), style|wsExLayered)
	}
	alpha := byte(opacity*255 + .5)
	procSetLayeredWindowAttributes.Call(uintptr(hwnd), 0, uintptr(alpha), lwaAlpha)
}

func nativeOpacityAvailable() bool { return true }

func moveOverlayToSavedPosition(hwnd windows.Handle, x, y int) {
	procSetWindowPos.Call(
		uintptr(hwnd),
		0,
		uintptr(int32(x)),
		uintptr(int32(y)),
		0,
		0,
		swpNoSize|swpNoZOrder|swpNoActivate,
	)
}

func windowRect(hwnd windows.Handle) (winRect, bool) {
	var rect winRect
	result, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	return rect, result != 0
}
