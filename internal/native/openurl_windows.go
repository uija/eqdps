//go:build windows

package native

import "os/exec"

func OpenURL(url string) error {
	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url).Start()
}
