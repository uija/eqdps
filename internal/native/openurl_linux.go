//go:build linux

package native

import "os/exec"

func OpenURL(url string) error {
	return exec.Command("xdg-open", url).Start()
}
