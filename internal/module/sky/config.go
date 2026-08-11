package sky

import "time"

type QuestItem struct {
	Name   string `json:"name"`
	Amount int    `json:"amount"`
}
type QuestDone struct {
	Name        string `json:"name"`
	Amount      int    `json:"amount"`
	Reactivated bool   `json:"reactivated"`
}
type Log struct {
	Path         string    `json:"path"`
	Offset       int64     `json:"offset"`
	LastTimezone time.Time `json:"last_timestamp"`
}

type Config struct {
	QuestItems []QuestItem `json:"quest_items"`
	Quests     []QuestDone `json:"quests"`
	Log        Log         `json:"log"`
}

func (c *Config) Load(path string) error {

	return nil
}

func (c *Config) Save() error {
	return nil
}
