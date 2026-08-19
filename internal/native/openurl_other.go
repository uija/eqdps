//go:build !linux && !windows && !darwin

package native

import (
	"fmt"
	"runtime"
)

func OpenURL(string) error {
	return fmt.Errorf("opening links is not supported on %s", runtime.GOOS)
}
