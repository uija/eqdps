package dps

import (
	"strconv"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/data"
)

type CombatDamageCategory struct {
	overall   *CombatDamageData
	abilities map[string]*CombatDamageData
}

func newCombatDamageCategory(name string) CombatDamageCategory {
	return CombatDamageCategory{
		overall:   newCombatDamageData(name),
		abilities: make(map[string]*CombatDamageData),
	}
}

func (d CombatDamageCategory) addDamageEvent(e *DamageEvent) {
	d.overall.addDamageEvent(e)
	if _, ok := d.abilities[e.Ability]; !ok {
		d.abilities[e.Ability] = newCombatDamageData(e.Ability)
	}
	d.abilities[e.Ability].addDamageEvent(e)
}

type CombatDamageData struct {
	name       string
	damage     int
	dps        float32
	sdps       float32
	hits       int
	crits      int
	minDamage  int
	maxDamage  int
	start      time.Time
	lastUpdate time.Time
}

func (d *CombatDamageData) addDamageEvent(e *DamageEvent) {
	if d.start.IsZero() {
		d.start = e.Time
	}
	d.lastUpdate = e.Time
	d.damage += e.Amount
	d.hits++
	if d.minDamage == 0 || d.minDamage > e.Amount {
		d.minDamage = e.Amount
	}
	if d.maxDamage < e.Amount {
		d.maxDamage = e.Amount
	}
	if e.Crit {
		d.crits++
	}
}
func activeDuration(start, end time.Time) time.Duration {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0
	}
	return end.Sub(start) + time.Second
}

func (d *CombatDamageData) DPS() float64 {
	duration := activeDuration(d.start, d.lastUpdate)
	if duration <= 0 {
		return 0
	}
	return float64(d.damage) / duration.Seconds()
}

func (d *CombatDamageData) SDPS(fightDuration time.Duration) float64 {
	if fightDuration <= 0 {
		return 0
	}
	return float64(d.damage) / fightDuration.Seconds()
}

func newCombatDamageData(name string) *CombatDamageData {
	return &CombatDamageData{
		name: name,
	}
}

type DamageEvent struct {
	Time             time.Time
	Source           string
	Target           string
	NormalizedSource string
	NormalizedTarget string
	Verb             string
	Amount           int
	Ability          string
	Crit             bool
	IsCast           bool
	Type             data.LogRowEventType
}

func (de *DamageEvent) isSpell() bool {
	return (de.Verb == "hit" || de.Verb == "hits") && de.Ability != ""
}

func NewDamageEvent(ts time.Time, source, target, verb, amount, ability, annotation string, t data.LogRowEventType) *DamageEvent {
	damage, err := strconv.Atoi(amount)
	if err != nil {
		damage = 0
	}
	return &DamageEvent{
		Time:             ts,
		Source:           source,
		NormalizedSource: normalizeName(source),
		Target:           target,
		NormalizedTarget: normalizeName(target),
		Verb:             verb,
		Amount:           damage,
		Ability:          ability,
		Crit:             strings.Contains(annotation, "Critical"),
		IsCast:           false,
		Type:             t,
	}
}
