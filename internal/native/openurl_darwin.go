//go:build darwin

package native

import "os/exec"

func OpenURL(url string) error {
	return exec.Command("open", url).Start()
}
