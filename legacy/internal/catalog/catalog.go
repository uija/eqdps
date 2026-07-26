package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed spells.json
var spellData []byte

type Spell struct {
	Name        string   `json:"name"`
	FadeMessage string   `json:"fade_message"`
	Classes     []string `json:"classes"`
	IconID      int      `json:"icon_id"`
}

func Load() ([]Spell, error) {
	var spells []Spell
	if err := json.Unmarshal(spellData, &spells); err != nil {
		return nil, fmt.Errorf("decode embedded spell catalogue: %w", err)
	}
	return spells, nil
}
