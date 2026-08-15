package dps

import (
	"log"
	"time"

	"gioui.org/widget"
	"github.com/uija/eqdps/internal/data"
)

const END_REASON_ZONED = "Zonedout"
const END_REASON_TIMEOUT = "Timeout"
const END_REASON_FD = "Feign Death"

const CATEGORY_MELEE = "Melee"
const CATEGORY_SPELLS = "Spells"
const CATEGORY_DOTS = "DoTs"
const CATEGORY_PROCS = "Procs"
const CATEGORY_DS = "Damage Shield"

var DamageCategories = []string{CATEGORY_MELEE, CATEGORY_SPELLS, CATEGORY_DOTS, CATEGORY_PROCS, CATEGORY_DS}

type Fight struct {
	name         string
	validated    bool
	participants map[string]bool

	combatants map[string]*Combatant

	start           time.Time
	lastUpdate      time.Time
	lastParticipate time.Time
	end             time.Time
	endReason       string
}

type Combatant struct {
	name       string
	normalized string

	overall    *CombatDamageData
	categories map[string]CombatDamageCategory

	open  bool
	click widget.Clickable
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
				category = CATEGORY_SPELLS
			} else {
				category = CATEGORY_PROCS
			}
		} else {
			category = CATEGORY_MELEE
		}
	case data.LogRowEventTypeDamageOverTime:
		category = CATEGORY_DOTS
	case data.LogRowEventTypeDamageShield:
		category = CATEGORY_DS
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
		f.end = e.Time
	}
	switch e.Type {
	case data.LogRowEventTypeDamageOverTime,
		data.LogRowEventTypeYourDamageOverTime,
		data.LogRowEventTypeDamageShield,
		data.LogRowEventTypeYourDamageShield:
	default:
		f.end = e.Time

	}
	f.participants[e.NormalizedSource] = true
	f.participants[e.NormalizedTarget] = true
	combatant, ok := f.combatants[e.NormalizedSource]
	if !ok {
		combatant = newCombatant(e.Source, e.NormalizedSource)
		f.combatants[e.NormalizedSource] = combatant
	}
	combatant.AddDamageEvent(e)
	if e.Participation {
		f.lastParticipate = e.Time
	}
}

func (f *Fight) Clone() *Fight {
	if f == nil {
		return nil
	}

	clone := &Fight{
		name:         f.name,
		validated:    f.validated,
		participants: make(map[string]bool, len(f.participants)),
		combatants:   make(map[string]*Combatant, len(f.combatants)),
		start:        f.start,
		lastUpdate:   f.lastUpdate,
		end:          f.end,
		endReason:    f.endReason,
	}

	for name, participant := range f.participants {
		clone.participants[name] = participant
	}

	for name, combatant := range f.combatants {
		clone.combatants[name] = combatant.Clone()
	}

	return clone
}

func (c *Combatant) Clone() *Combatant {
	if c == nil {
		return nil
	}

	clone := &Combatant{
		name:       c.name,
		normalized: c.normalized,
		overall:    cloneDamageData(c.overall),
		categories: make(map[string]CombatDamageCategory, len(c.categories)),
	}

	for name, category := range c.categories {
		categoryClone := CombatDamageCategory{
			overall:   cloneDamageData(category.overall),
			abilities: make(map[string]*CombatDamageData, len(category.abilities)),
		}

		for ability, damage := range category.abilities {
			categoryClone.abilities[ability] = cloneDamageData(damage)
		}

		clone.categories[name] = categoryClone
	}

	return clone
}

func cloneDamageData(data *CombatDamageData) *CombatDamageData {
	if data == nil {
		return nil
	}

	clone := *data
	return &clone
}
