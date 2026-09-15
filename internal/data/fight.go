package data

import (
	"log"
	"strings"
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

const HEAL_OVER_TIME = "Heal over Time"
const DIRECT_HEAL = "Direct Heal"

var DamageCategories = []string{CATEGORY_MELEE, CATEGORY_SPELLS, CATEGORY_DOTS, CATEGORY_PROCS, CATEGORY_DS}
var HealingCategories = []string{DIRECT_HEAL, HEAL_OVER_TIME}

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

	Overall           *CombatDamageData
	OverallHealing    *HealingData
	Categories        map[string]CombatDamageCategory
	HealingCategories map[string]HealingCategory

	FirstParticipation time.Time

	Open  bool
	Click widget.Clickable
}

func NewCombatant(name, normalized string) *Combatant {
	return &Combatant{
		Name:              name,
		Normalized:        normalized,
		Overall:           NewCombatDamageData(name),
		Categories:        make(map[string]CombatDamageCategory),
		OverallHealing:    NewHealingData(name),
		HealingCategories: make(map[string]HealingCategory),
	}
}
func (c *Combatant) AddHealing(ability string, amount int64, hot bool, crit bool) {
	c.OverallHealing.AddHealing(amount, crit)
	catname := DIRECT_HEAL
	if hot {
		catname = HEAL_OVER_TIME
	}
	if _, ok := c.HealingCategories[catname]; !ok {
		c.HealingCategories[catname] = NewHealingCategory(catname)
	}
	c.HealingCategories[catname].AddHealing(ability, amount, crit)
}

func (c *Combatant) AddDamageEvent(e *DamageEvent) {
	if e.Participation && c.FirstParticipation.IsZero() {
		c.FirstParticipation = e.Time
	}
	c.Overall.AddDamageEvent(e, !c.FirstParticipation.IsZero())

	category := ""
	switch e.Type {
	case LogRowEventTypeFailedMelee,
		LogRowEventTypeFailedMeleeOthers:
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
func (f *Fight) AddHealing(source string, target string, ability string, amount int, hot bool, crit bool) {
	normalizedSource := strings.ToLower(strings.TrimSpace(source))

	combatant, ok := f.Combatants[normalizedSource]
	if !ok {
		combatant = NewCombatant(source, normalizedSource)
		f.Combatants[normalizedSource] = combatant
	}
	combatant.AddHealing(ability, int64(amount), hot, crit)
}
func (f *Fight) AddDamageEvent(e *DamageEvent, autoOpenYou bool) {
	if f.Start.IsZero() {
		f.Start = e.Time
		f.End = e.Time
	}
	f.LastUpdate = e.Time
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
		/*
			if autoOpenYou && strings.EqualFold(e.Source, "You") {
				combatant.Open = true
			}
		*/
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
