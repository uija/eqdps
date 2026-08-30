package data

import (
	"strconv"
	"strings"
	"time"
)

type CombatDamageCategory struct {
	Overall   *CombatDamageData
	Abilities map[string]*CombatDamageData
}

func NewCombatDamageCategory(name string) CombatDamageCategory {
	return CombatDamageCategory{
		Overall:   NewCombatDamageData(name),
		Abilities: make(map[string]*CombatDamageData),
	}
}

func (d CombatDamageCategory) AddDamageEvent(e *DamageEvent) {
	d.Overall.AddDamageEvent(e, false)
	if _, ok := d.Abilities[e.Ability]; !ok {
		d.Abilities[e.Ability] = NewCombatDamageData(e.Ability)
	}
	d.Abilities[e.Ability].AddDamageEvent(e, false)
}

type CombatDamageData struct {
	Name         string
	Damage       int
	ActiveDamage int
	Dps          float32
	Sdps         float32
	Hits         int
	Miss         int
	Dodge        int
	Parry        int
	Block        int
	Absorb       int
	Riposte      int
	Crits        int
	MinDamage    int
	MaxDamage    int
	Start        time.Time
	LastUpdate   time.Time
}

func (d *CombatDamageData) NumAttacks() int {
	return d.Hits + d.Miss + d.Dodge + d.Parry + d.Block + d.Absorb
}
func (d *CombatDamageData) AddDamageEvent(e *DamageEvent, active_damage bool) {
	if d.Start.IsZero() {
		d.Start = e.Time
	}
	d.LastUpdate = e.Time

	switch e.Result {
	case AttackResultMiss:
		d.Miss++
		return
	case AttackResultDodge:
		d.Dodge++
		return
	case AttackResultParry:
		d.Parry++
		return
	case AttackResultBlock:
		d.Block++
		return
	case AttackResultAbsorb:
		d.Absorb++
		return
	case AttackResultRiposte:
		d.Riposte++
		return
	}
	d.Hits++

	d.Damage += e.Amount
	if active_damage {
		d.ActiveDamage += e.Amount
	}
	if d.MinDamage == 0 || d.MinDamage > e.Amount {
		d.MinDamage = e.Amount
	}
	if d.MaxDamage < e.Amount {
		d.MaxDamage = e.Amount
	}
	if e.Crit {
		d.Crits++
	}
}
func ActiveDuration(start, end time.Time) time.Duration {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0
	}
	return end.Sub(start) + time.Second
}

func (d *CombatDamageData) DPS() float64 {
	duration := ActiveDuration(d.Start, d.LastUpdate)
	if duration <= 0 {
		return 0
	}
	return float64(d.Damage) / duration.Seconds()
}

func (d *CombatDamageData) SDPS(fightDuration time.Duration) float64 {
	if fightDuration <= 0 {
		return 0
	}
	if d.ActiveDamage == 0 {
		return 0
	}
	return float64(d.ActiveDamage) / fightDuration.Seconds()
}

func NewCombatDamageData(name string) *CombatDamageData {
	return &CombatDamageData{
		Name: name,
	}
}

type AttackResult uint8

const (
	AttackResultHit AttackResult = iota
	AttackResultMiss
	AttackResultDodge
	AttackResultParry
	AttackResultBlock
	AttackResultAbsorb
	AttackResultRiposte
)

type DamageEvent struct {
	Result           AttackResult
	Time             time.Time
	Source           string
	Target           string
	NormalizedSource string
	NormalizedTarget string
	Verb             string
	Amount           int
	Ability          string
	Crit             bool
	Riposte          bool
	IsCast           bool
	Type             LogRowEventType
	Participation    bool
}

func (de *DamageEvent) IsSpell() bool {
	return (de.Verb == "hit" || de.Verb == "hits") && de.Ability != ""
}

func NewDamageEvent(result AttackResult, ts time.Time, source, target, verb, amount, ability, annotation string, t LogRowEventType) *DamageEvent {
	damage, err := strconv.Atoi(amount)
	if err != nil {
		damage = 0
	}
	return &DamageEvent{
		Result:           result,
		Time:             ts,
		Source:           source,
		NormalizedSource: normalizeName(source),
		Target:           target,
		NormalizedTarget: normalizeName(target),
		Verb:             verb,
		Amount:           damage,
		Ability:          ability,
		Crit:             strings.Contains(annotation, "Critical"),
		Riposte:          strings.Contains(annotation, "Riposte"),
		IsCast:           false,
		Type:             t,
		Participation:    false,
	}
}
func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
