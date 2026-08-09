package dps

import (
	"log"
	"time"

	"github.com/uija/eqdps/internal/data"
)

type Fight struct {
	name         string
	validated    bool
	participants map[string]bool

	combatants map[string]*Combatant

	start      time.Time
	lastUpdate time.Time
	end        time.Time
	endReason  string
}

type Combatant struct {
	name       string
	normalized string

	overall    *CombatDamageData
	categories map[string]CombatDamageCategory
}

func newCombatant(name, normalized string) *Combatant {
	return &Combatant{
		name:       name,
		normalized: normalized,
		overall:    newCombatDamageData(name),
		categories: make(map[string]CombatDamageCategory),
	}
}
func (c *Combatant) AddDamageEvent(e *DamageEvent) {
	c.overall.addDamageEvent(e)

	category := ""
	switch e.Type {
	case data.LogRowEventTypeDamage:
		if e.isSpell() {
			if e.IsCast {
				category = "Spells"
			} else {
				category = "Procs"
			}
		} else {
			category = "Melee"
		}
	case data.LogRowEventTypeDamageOverTime:
		category = "DoTs"
	case data.LogRowEventTypeDamageShield:
		category = "Damage Shield"
	}
	if category == "" {
		log.Printf("Unable to determine category.")
		return
	}
	if _, ok := c.categories[category]; !ok {
		c.categories[category] = newCombatDamageCategory(category)
	}
	c.categories[category].addDamageEvent(e)
}

func newFight(validated bool) *Fight {
	return &Fight{
		validated:    validated,
		participants: make(map[string]bool),
		combatants:   make(map[string]*Combatant),
	}
}
func (f *Fight) hasParticipant(name string) bool {
	_, ok := f.participants[name]
	return ok
}
func (f *Fight) addDamageEvent(e *DamageEvent) {
	if f.start.IsZero() {
		f.start = e.Time
	}
	f.end = e.Time
	f.participants[e.NormalizedSource] = true
	f.participants[e.NormalizedTarget] = true
	combatant, ok := f.combatants[e.NormalizedSource]
	if !ok {
		combatant = newCombatant(e.Source, e.NormalizedSource)
		f.combatants[e.NormalizedSource] = combatant
	}
	combatant.AddDamageEvent(e)
}
