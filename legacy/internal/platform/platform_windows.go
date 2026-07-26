package platform

import (
	"os/exec"
	"strings"
)

func OpenURL(url string) error {
	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url).Start()
}

func CopyText(text string) error {
	command := exec.Command("clip.exe")
	command.Stdin = strings.NewReader(text)
	return command.Run()
}
