//go:build windows

package overlay

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
	hwndTopmost   = ^uintptr(0)
	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
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

func handleNativeOverlayEvent(overlay *Overlay, event any) {
	view, ok := event.(app.Win32ViewEvent)
	if !ok || view.HWND == 0 {
		return
	}
	overlay.nativeMu.Lock()
	config := overlay.Config()
	overlay.nativeHandle = view.HWND
	hasSavedPosition := func() bool {
		if config.UIConfig.OverlayX != 0 && config.UIConfig.OverlayY != 0 {
			return true
		}
		return false
	}()
	restorePosition := hasSavedPosition && !overlay.positionRestored
	if restorePosition {
		overlay.positionRestored = true
	}
	x, y := config.UIConfig.OverlayX, config.UIConfig.OverlayY
	overlay.nativeMu.Unlock()
	go func(handle uintptr) {
		time.Sleep(100 * time.Millisecond)
		if restorePosition {
			moveOverlayToSavedPosition(windows.Handle(handle), x, y)
		} else {
			enforceTopMost(windows.Handle(handle))
		}
		applyDesiredNativeOverlayOpacity(overlay, handle)
	}(view.HWND)
}

func captureNativeOverlayPosition(overlay *Overlay) {
	overlay.nativeMu.Lock()
	defer overlay.nativeMu.Unlock()
	if overlay.nativeHandle == 0 {
		return
	}
	hwnd := windows.Handle(overlay.nativeHandle)
	if rect, ok := windowRect(hwnd); ok {
		overlay.Config().UIConfig.OverlayX = int(rect.left)
		overlay.Config().UIConfig.OverlayY = int(rect.top)
		overlay.Config().Save()
	}
}

func setNativeOverlayOpacity(overlay *Overlay, opacity float32) {
	overlay.nativeMu.Lock()
	defer overlay.nativeMu.Unlock()
	if overlay.nativeHandle != 0 {
		applyWindowOpacity(windows.Handle(overlay.nativeHandle), opacity)
	}
}

func applyDesiredNativeOverlayOpacity(overlay *Overlay, handle uintptr) {
	overlay.nativeMu.Lock()
	defer overlay.nativeMu.Unlock()
	if overlay.nativeHandle == handle {
		applyWindowOpacity(windows.Handle(handle), overlay.Config().UIConfig.OverlayOpacity)
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
		hwndTopmost,
		uintptr(int32(x)),
		uintptr(int32(y)),
		0,
		0,
		swpNoSize|swpNoActivate,
	)
}
func enforceTopMost(hwnd windows.Handle) {
	procSetWindowPos.Call(
		uintptr(hwnd),
		hwndTopmost,
		0,
		0,
		0,
		0,
		swpNoMove|swpNoSize|swpNoActivate,
	)
}

func windowRect(hwnd windows.Handle) (winRect, bool) {
	var rect winRect
	result, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	return rect, result != 0
}
