package statistics

import (
	"encoding/json"
	"fmt"
	"os"
)

type AbilityStat struct {
	Count  int64 `json:"count"`
	Damage int64 `json:"damage"`
}
type DamageTracker struct {
	Min        int    `json:"min"`
	Max        int    `json:"max"`
	MinWho     string `json:"min_who"`
	MinAbility string `json:"min_ability"`
	MaxWho     string `json:"max_who"`
	MaxAbility string `json:"max_ability"`
}
type Log struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
}
type Config struct {
	Mobs      map[string]int64       `json:"mobs"`
	Zones     map[string]int64       `json:"zones"`
	Spells    map[string]int64       `json:"spells"`
	Abilities map[string]AbilityStat `json:"abilitits"`
	Items     map[string]int64       `json:"items"`
	DamageInc DamageTracker          `json:"damage_inc"`
	DamageOut DamageTracker          `json:"damage_out"`

	Log Log `json:"log"`
}

func LoadConfig(path string) (Config, error) {
	config := Config{
		Mobs:      make(map[string]int64),
		Zones:     make(map[string]int64),
		Spells:    make(map[string]int64),
		Abilities: make(map[string]AbilityStat),
		Items:     make(map[string]int64),
	}
	config.Log.Path = path
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, err
	}
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}
func (c *Config) Save() error {
	if c.Log.Path == "" {
		return fmt.Errorf("No path defined for config.")
	}
	bytes, err := json.Marshal(c)
	if err != nil {
		return err
	}
	err = os.WriteFile(c.Log.Path, bytes, 0644)
	return err
}
