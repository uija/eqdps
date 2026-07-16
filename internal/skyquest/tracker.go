package skyquest

import (
	"strings"

	"github.com/uija/eqdps/internal/eqlog"
)

const PlaneOfSkyZone = "The Plane of Sky"

type Tracker struct {
	database Database
	known    map[string]struct{}
	owned    map[string]int
	zone     string
}

type QuestProgress struct {
	Class   string
	Quest   Quest
	Missing []Requirement
	Ready   bool
}

func NewTracker(database Database) *Tracker {
	tracker := &Tracker{
		database: database,
		known:    make(map[string]struct{}),
		owned:    make(map[string]int),
	}
	for _, class := range database.Classes {
		for _, quest := range class.Quests {
			for _, requirement := range quest.Requirements {
				tracker.known[requirement.Name] = struct{}{}
			}
		}
	}
	return tracker
}

func (t *Tracker) ProcessRecord(record eqlog.Record) {
	switch record.Kind {
	case eqlog.RecordZoneChange:
		t.zone = strings.TrimSpace(record.ZoneChange.Name)
	case eqlog.RecordLoot:
		t.addLoot(record.Loot)
	case eqlog.RecordItemRemoval:
		t.remove(record.Removal.Item, record.Removal.Quantity)
	}
}

func (t *Tracker) addLoot(loot eqlog.Loot) {
	if t.zone != PlaneOfSkyZone {
		return
	}
	if _, ok := t.known[loot.Item]; !ok {
		return
	}
	if loot.Outcome != eqlog.LootRetained && loot.Outcome != eqlog.LootStored {
		return
	}
	t.owned[loot.Item] += loot.Quantity
}

func (t *Tracker) remove(item string, quantity int) {
	if _, ok := t.known[item]; !ok || quantity < 1 {
		return
	}
	t.owned[item] -= quantity
	if t.owned[item] <= 0 {
		delete(t.owned, item)
	}
}

func (t *Tracker) Zone() string {
	return t.zone
}

func (t *Tracker) Owned(item string) int {
	return t.owned[item]
}

func (t *Tracker) Inventory() map[string]int {
	result := make(map[string]int, len(t.owned))
	for item, quantity := range t.owned {
		result[item] = quantity
	}
	return result
}

func (t *Tracker) QuestProgress() []QuestProgress {
	var result []QuestProgress
	for _, class := range t.database.Classes {
		for _, quest := range class.Quests {
			progress := QuestProgress{Class: class.Name, Quest: quest}
			for _, requirement := range quest.Requirements {
				if t.owned[requirement.Name] < requirement.Quantity {
					progress.Missing = append(progress.Missing, requirement)
				}
			}
			progress.Ready = len(progress.Missing) == 0
			result = append(result, progress)
		}
	}
	return result
}

func (t *Tracker) ReadyQuests() []QuestProgress {
	var result []QuestProgress
	for _, progress := range t.QuestProgress() {
		if progress.Ready {
			result = append(result, progress)
		}
	}
	return result
}
