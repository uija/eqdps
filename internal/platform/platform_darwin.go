package platform

import (
	"os/exec"
	"strings"
)

func OpenURL(url string) error {
	return exec.Command("open", url).Start()
}

func CopyText(text string) error {
	command := exec.Command("pbcopy")
	command.Stdin = strings.NewReader(text)
	return command.Run()
}
