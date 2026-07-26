package audio

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

const (
	EmbeddedPrefix = "embedded:"
	UserPrefix     = "user:"
)

//go:embed assets/*.mp3
var embeddedFiles embed.FS

type Sound struct {
	ID    string
	Label string
}

func EmbeddedSounds() ([]Sound, error) {
	entries, err := fs.ReadDir(embeddedFiles, "assets")
	if err != nil {
		return nil, fmt.Errorf("read embedded sounds: %w", err)
	}

	var sounds []Sound
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(path.Ext(entry.Name()), ".mp3") {
			continue
		}
		sounds = append(sounds, Sound{
			ID:    EmbeddedPrefix + entry.Name(),
			Label: entry.Name() + " (embedded)",
		})
	}
	sort.Slice(sounds, func(i, j int) bool {
		return sounds[i].Label < sounds[j].Label
	})
	return sounds, nil
}

func ReadEmbedded(id string) ([]byte, error) {
	name, ok := strings.CutPrefix(id, EmbeddedPrefix)
	if !ok || name == "" || path.Base(name) != name {
		return nil, fmt.Errorf("invalid embedded sound ID %q", id)
	}
	data, err := embeddedFiles.ReadFile(path.Join("assets", name))
	if err != nil {
		return nil, fmt.Errorf("read embedded sound %q: %w", name, err)
	}
	return data, nil
}
