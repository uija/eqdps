package data

import (
	"log"
	"time"

	"gioui.org/widget"
)

const END_REASON_ZONED = "Zonedout"
const END_REASON_TIMEOUT = "Timeout"
const END_REASON_FD = "Feign Death"
const END_REASON_DEATH = "Death"

const CATEGORY_MELEE = "Melee"
const CATEGORY_SPELLS = "Spells"
const CATEGORY_DOTS = "DoTs"
const CATEGORY_PROCS = "Procs"
const CATEGORY_DS = "Damage Shield"

var DamageCategories = []string{CATEGORY_MELEE, CATEGORY_SPELLS, CATEGORY_DOTS, CATEGORY_PROCS, CATEGORY_DS}

type Fight struct {
	Name         string
	Validated    bool
	Participants map[string]bool

	Combatants map[string]*Combatant

	Start           time.Time
	LastUpdate      time.Time
	LastParticipate time.Time
	End             time.Time
	EndReason       string
}

type Combatant struct {
	Name       string
	Normalized string

	Overall    *CombatDamageData
	Categories map[string]CombatDamageCategory

	FirstParticipation time.Time

	Open  bool
	Click widget.Clickable
}

func NewCombatant(name, normalized string) *Combatant {
	return &Combatant{
		Name:       name,
		Normalized: normalized,
		Overall:    NewCombatDamageData(name),
		Categories: make(map[string]CombatDamageCategory),
	}
}

func (c *Combatant) AddDamageEvent(e *DamageEvent) {
	if e.Participation && c.FirstParticipation.IsZero() {
		c.FirstParticipation = e.Time
	}
	c.Overall.AddDamageEvent(e, !c.FirstParticipation.IsZero())

	category := ""
	switch e.Type {
	case LogRowEventTypeFailedMelee:
		category = CATEGORY_MELEE
	case LogRowEventTypeDamage:
		if e.IsSpell() {
			if e.IsCast {
				category = CATEGORY_SPELLS
			} else {
				category = CATEGORY_PROCS
			}
		} else {
			category = CATEGORY_MELEE
		}
	case LogRowEventTypeDamageOverTime:
		category = CATEGORY_DOTS
	case LogRowEventTypeDamageShield:
		category = CATEGORY_DS
	}
	if category == "" {
		log.Printf("Unable to determine category.")
		return
	}
	if _, ok := c.Categories[category]; !ok {
		c.Categories[category] = NewCombatDamageCategory(category)
	}
	c.Categories[category].AddDamageEvent(e)
}

func NewFight(validated bool) *Fight {
	return &Fight{
		Validated:    validated,
		Participants: make(map[string]bool),
		Combatants:   make(map[string]*Combatant),
	}
}
func (f *Fight) HasParticipant(name string) bool {
	_, ok := f.Participants[name]
	return ok
}
func (f *Fight) AddDamageEvent(e *DamageEvent) {
	if f.Start.IsZero() {
		f.Start = e.Time
		f.End = e.Time
	}
	switch e.Type {
	case LogRowEventTypeDamageOverTime,
		LogRowEventTypeYourDamageOverTime,
		LogRowEventTypeDamageShield,
		LogRowEventTypeYourDamageShield:
	default:
		f.End = e.Time

	}
	f.Participants[e.NormalizedSource] = true
	f.Participants[e.NormalizedTarget] = true
	combatant, ok := f.Combatants[e.NormalizedSource]
	if !ok {
		combatant = NewCombatant(e.Source, e.NormalizedSource)
		f.Combatants[e.NormalizedSource] = combatant
	}
	if e.Participation {
		f.LastParticipate = e.Time
	}
	combatant.AddDamageEvent(e)
}

func (f *Fight) Clone() *Fight {
	if f == nil {
		return nil
	}

	clone := &Fight{
		Name:         f.Name,
		Validated:    f.Validated,
		Participants: make(map[string]bool, len(f.Participants)),
		Combatants:   make(map[string]*Combatant, len(f.Combatants)),
		Start:        f.Start,
		LastUpdate:   f.LastUpdate,
		End:          f.End,
		EndReason:    f.EndReason,
	}

	for name, participant := range f.Participants {
		clone.Participants[name] = participant
	}

	for name, combatant := range f.Combatants {
		clone.Combatants[name] = combatant.Clone()
	}

	return clone
}

func (c *Combatant) Clone() *Combatant {
	if c == nil {
		return nil
	}

	clone := &Combatant{
		Name:               c.Name,
		Normalized:         c.Normalized,
		Overall:            CloneDamageData(c.Overall),
		Categories:         make(map[string]CombatDamageCategory, len(c.Categories)),
		FirstParticipation: c.FirstParticipation,
	}

	for name, category := range c.Categories {
		categoryClone := CombatDamageCategory{
			Overall:   CloneDamageData(category.Overall),
			Abilities: make(map[string]*CombatDamageData, len(category.Abilities)),
		}

		for ability, damage := range category.Abilities {
			categoryClone.Abilities[ability] = CloneDamageData(damage)
		}

		clone.Categories[name] = categoryClone
	}

	return clone
}

func CloneDamageData(data *CombatDamageData) *CombatDamageData {
	if data == nil {
		return nil
	}

	clone := *data
	return &clone
}
