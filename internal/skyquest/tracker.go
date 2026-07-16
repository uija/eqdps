package skyquest

import (
	"strings"

	"github.com/uija/eqdps/internal/eqlog"
)

const PlaneOfSkyZone = "The Plane of Sky"

type Tracker struct {
	database  Database
	known     map[string]struct{}
	owned     map[string]int
	completed map[string]bool
	pending   map[string]map[string]int
	zone      string
}

type QuestProgress struct {
	Class     string
	Quest     Quest
	Missing   []Requirement
	Ready     bool
	Completed bool
}

func NewTracker(database Database) *Tracker {
	tracker := &Tracker{
		database:  database,
		known:     make(map[string]struct{}),
		owned:     make(map[string]int),
		completed: make(map[string]bool),
		pending:   make(map[string]map[string]int),
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
		t.pending = make(map[string]map[string]int)
	case eqlog.RecordLoot:
		t.addLoot(record.Loot)
	case eqlog.RecordItemRemoval:
		t.remove(record.Removal.Item, record.Removal.Quantity)
	case eqlog.RecordTradeOffer:
		t.offer(record.TradeOffer)
	case eqlog.RecordTradeComplete:
		t.completeTrade(record.TradeDone)
	}
}

func (t *Tracker) offer(offer eqlog.TradeOffer) {
	if t.zone != PlaneOfSkyZone {
		return
	}
	if _, known := t.known[offer.Item]; !known {
		return
	}
	if t.pending[offer.NPC] == nil {
		t.pending[offer.NPC] = make(map[string]int)
	}
	t.pending[offer.NPC][offer.Item] += offer.Quantity
}

func (t *Tracker) completeTrade(completed eqlog.TradeComplete) {
	offered := t.pending[completed.NPC]
	delete(t.pending, completed.NPC)
	if t.zone != PlaneOfSkyZone || len(offered) == 0 {
		return
	}
	for _, class := range t.database.Classes {
		for _, quest := range class.Quests {
			if quest.QuestGiver != completed.NPC || t.completed[quest.Name] || !sameRequirements(offered, quest.Requirements) {
				continue
			}
			t.completed[quest.Name] = true
			for _, requirement := range quest.Requirements {
				t.remove(requirement.Name, requirement.Quantity)
			}
			return
		}
	}
}

func sameRequirements(offered map[string]int, requirements []Requirement) bool {
	wanted := make(map[string]int, len(requirements))
	for _, requirement := range requirements {
		wanted[requirement.Name] += requirement.Quantity
	}
	if len(offered) != len(wanted) {
		return false
	}
	for item, quantity := range wanted {
		if offered[item] != quantity {
			return false
		}
	}
	return true
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

func (t *Tracker) Completed() map[string]bool {
	result := make(map[string]bool, len(t.completed))
	for quest, completed := range t.completed {
		if completed {
			result[quest] = true
		}
	}
	return result
}

func (t *Tracker) PendingOffers() map[string]map[string]int {
	result := make(map[string]map[string]int, len(t.pending))
	for npc, offered := range t.pending {
		result[npc] = make(map[string]int, len(offered))
		for item, quantity := range offered {
			result[npc][item] = quantity
		}
	}
	return result
}

func (t *Tracker) QuestProgress() []QuestProgress {
	var result []QuestProgress
	for _, class := range t.database.Classes {
		for _, quest := range class.Quests {
			progress := QuestProgress{Class: class.Name, Quest: quest, Completed: t.completed[quest.Name]}
			for _, requirement := range quest.Requirements {
				if t.owned[requirement.Name] < requirement.Quantity {
					progress.Missing = append(progress.Missing, requirement)
				}
			}
			progress.Ready = !progress.Completed && len(progress.Missing) == 0
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
