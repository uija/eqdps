// Package skyquest provides the embedded Plane of Sky class quest database.
package skyquest

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed plane_of_sky_quests.json
var databaseJSON []byte

type Database struct {
	SchemaVersion  int     `json:"schema_version"`
	Source         string  `json:"source"`
	SourcePageID   int     `json:"source_page_id"`
	SourceRevision int     `json:"source_revision"`
	Classes        []Class `json:"classes"`
}

type Class struct {
	Name           string  `json:"name"`
	QuestNPC       string  `json:"quest_npc"`
	Source         string  `json:"source,omitempty"`
	SourcePageID   int     `json:"source_page_id,omitempty"`
	SourceRevision int     `json:"source_revision,omitempty"`
	Quests         []Quest `json:"quests"`
}

type Quest struct {
	Name         string        `json:"name"`
	QuestGiver   string        `json:"quest_giver"`
	Requirements []Requirement `json:"requirements"`
	Rewards      []string      `json:"rewards"`
}

type Requirement struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Quantity   int    `json:"quantity"`
	NoDrop     bool   `json:"no_drop,omitempty"`
	Island     int    `json:"island,omitempty"`
	DropsFrom  string `json:"drops_from,omitempty"`
	SourceHint string `json:"source_hint,omitempty"`
}

func LoadDatabase() (Database, error) {
	var database Database
	if err := json.Unmarshal(databaseJSON, &database); err != nil {
		return Database{}, fmt.Errorf("decode embedded Plane of Sky quest database: %w", err)
	}
	if database.SchemaVersion != 1 {
		return Database{}, fmt.Errorf("unsupported Plane of Sky quest database schema %d", database.SchemaVersion)
	}
	return database, nil
}
