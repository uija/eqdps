//go:build !linux && !windows && !darwin

package platform

import (
	"fmt"
	"runtime"
)

func OpenURL(string) error {
	return fmt.Errorf("opening links is not supported on %s", runtime.GOOS)
}

func CopyText(string) error {
	return fmt.Errorf("copying text is not supported on %s", runtime.GOOS)
}
