package platform

import (
	"fmt"
	"os/exec"
	"strings"
)

func OpenURL(url string) error {
	return exec.Command("xdg-open", url).Start()
}

func CopyText(text string) error {
	commands := []struct {
		name string
		args []string
	}{
		{name: "wl-copy"},
		{name: "xclip", args: []string{"-selection", "clipboard"}},
		{name: "xsel", args: []string{"--clipboard", "--input"}},
	}
	for _, candidate := range commands {
		path, err := exec.LookPath(candidate.name)
		if err != nil {
			continue
		}
		command := exec.Command(path, candidate.args...)
		command.Stdin = strings.NewReader(text)
		if err := command.Run(); err != nil {
			return fmt.Errorf("copy with %s: %w", candidate.name, err)
		}
		return nil
	}
	return fmt.Errorf("no clipboard helper found; install wl-clipboard, xclip, or xsel")
}
