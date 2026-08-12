package sky

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Log struct {
	Path          string    `json:"path"`
	Offset        int64     `json:"offset"`
	LastTimestamp time.Time `json:"last_timestamp"`
}

type Config struct {
	QuestItems map[string]int `json:"quest_items"`
	Quests     map[string]int `json:"quests"`
	Log        Log            `json:"log"`
}

func LoadConfig(path string) (Config, error) {
	bytes, err := os.ReadFile(path)
	config := Config{
		QuestItems: make(map[string]int),
		Quests:     make(map[string]int),
		Log: Log{
			Path:          path,
			Offset:        0,
			LastTimestamp: time.Time{},
		},
	}
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

func (c *Config) HasItem(name string) int {
	amount, ok := c.QuestItems[name]
	if !ok {
		return 0
	}
	return amount
}
