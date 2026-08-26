package audio

import (
	"os"
	"path/filepath"
)

func AudioPath() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err == nil {
		configDir := filepath.Join(baseDir, "eqdps")
		audioDir := filepath.Join(configDir, "audio")
		if err := os.MkdirAll(audioDir, 0o700); err != nil {
			return "", err
		}
		return audioDir, nil
	}
	return "", err
}
