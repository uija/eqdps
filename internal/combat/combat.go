package combat

import (
	"sort"
	"strings"
	"time"
)

const deathGracePeriod = 8 * time.Second
const DefaultIdleTimeout = 15 * time.Second
const DefaultFightHistory = 0

type Event struct {
	Time     time.Time
	Source   string
	Target   string
	Amount   int
	Kind     string
	Ability  string
	Critical bool
	Passive  bool
}

type Death struct {
	Time   time.Time
	Victim string
	Killer string
}

type PlayerStats struct {
	Name        string
	Damage      int
	Hits        int
	Crits       int
	FirstSeen   time.Time
	LastSeen    time.Time
	LastTarget  string
	DamageTypes map[string]int
}

func (s PlayerStats) ActiveDuration() time.Duration {
	if s.FirstSeen.IsZero() || s.LastSeen.IsZero() {
		return 0
	}
	if s.LastSeen.Before(s.FirstSeen) {
		return 0
	}
	return s.LastSeen.Sub(s.FirstSeen) + time.Second
}

func (s PlayerStats) DPS() float64 {
	seconds := s.ActiveDuration().Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(s.Damage) / seconds
}

type Meter struct {
	players map[string]*PlayerStats
	events  int
	started time.Time
	ended   time.Time
}

func NewMeter() *Meter {
	return &Meter{players: make(map[string]*PlayerStats)}
}

func (m *Meter) Add(event Event) {
	if event.Amount <= 0 || event.Source == "" {
		return
	}

	stats := m.players[event.Source]
	if stats == nil {
		stats = &PlayerStats{
			Name:        event.Source,
			FirstSeen:   event.Time,
			DamageTypes: make(map[string]int),
		}
		m.players[event.Source] = stats
	}

	stats.Damage += event.Amount
	stats.DamageTypes[damageType(event)] += event.Amount
	stats.Hits++
	if event.Critical {
		stats.Crits++
	}
	if stats.FirstSeen.IsZero() || event.Time.Before(stats.FirstSeen) {
		stats.FirstSeen = event.Time
	}
	if stats.LastSeen.IsZero() || event.Time.After(stats.LastSeen) {
		stats.LastSeen = event.Time
	}
	stats.LastTarget = event.Target
	m.events++
	if m.started.IsZero() || event.Time.Before(m.started) {
		m.started = event.Time
	}
	if m.ended.IsZero() || event.Time.After(m.ended) {
		m.ended = event.Time
	}
}

func (m *Meter) Events() int {
	return m.events
}

func (m *Meter) Started() time.Time {
	return m.started
}

func (m *Meter) Ended() time.Time {
	return m.ended
}

func (m *Meter) Players() []PlayerStats {
	merged := make(map[string]*PlayerStats, len(m.players))
	for _, stats := range m.players {
		if owner, petName, ok := possessiveOwner(stats.Name); ok {
			if _, ownerExists := m.players[owner]; ownerExists {
				ownerStats := merged[owner]
				if ownerStats == nil {
					ownerStats = copyStats(m.players[owner])
					merged[owner] = ownerStats
				}
				mergePetStats(ownerStats, stats, petName)
				continue
			}
		}
		if _, exists := merged[stats.Name]; !exists {
			merged[stats.Name] = copyStats(stats)
		}
	}

	players := make([]PlayerStats, 0, len(merged))
	for _, stats := range merged {
		players = append(players, *stats)
	}

	sort.Slice(players, func(i, j int) bool {
		if players[i].FirstSeen.Equal(players[j].FirstSeen) {
			return players[i].Name < players[j].Name
		}
		return players[i].FirstSeen.Before(players[j].FirstSeen)
	})

	return players
}

func copyStats(stats *PlayerStats) *PlayerStats {
	if stats == nil {
		return nil
	}
	copied := *stats
	copied.DamageTypes = make(map[string]int, len(stats.DamageTypes))
	for name, amount := range stats.DamageTypes {
		copied.DamageTypes[name] = amount
	}
	return &copied
}

func mergePetStats(owner, pet *PlayerStats, petName string) {
	owner.Damage += pet.Damage
	owner.Hits += pet.Hits
	owner.Crits += pet.Crits
	if owner.FirstSeen.IsZero() || (!pet.FirstSeen.IsZero() && pet.FirstSeen.Before(owner.FirstSeen)) {
		owner.FirstSeen = pet.FirstSeen
	}
	if owner.LastSeen.IsZero() || pet.LastSeen.After(owner.LastSeen) {
		owner.LastSeen = pet.LastSeen
		owner.LastTarget = pet.LastTarget
	}
	if owner.DamageTypes == nil {
		owner.DamageTypes = make(map[string]int)
	}
	owner.DamageTypes["Pet: "+petName] += pet.Damage
}

func possessiveOwner(name string) (string, string, bool) {
	for _, separator := range []string{"`s ", "'s "} {
		owner, petName, ok := strings.Cut(name, separator)
		if ok && owner != "" && petName != "" {
			return owner, petName, true
		}
	}
	return "", "", false
}

func (s PlayerStats) DamageBreakdown() []DamageBreakdown {
	breakdown := make([]DamageBreakdown, 0, len(s.DamageTypes))
	for name, amount := range s.DamageTypes {
		breakdown = append(breakdown, DamageBreakdown{Name: name, Damage: amount})
	}
	sort.Slice(breakdown, func(i, j int) bool {
		if breakdown[i].Damage == breakdown[j].Damage {
			return breakdown[i].Name < breakdown[j].Name
		}
		return breakdown[i].Damage > breakdown[j].Damage
	})
	return breakdown
}

type DamageBreakdown struct {
	Name   string
	Damage int
}

func damageType(event Event) string {
	if event.Ability != "" {
		return event.Ability
	}
	return "Melee"
}

type Fight struct {
	Meter     *Meter
	Death     Death
	EndReason string
}

type DisplaySection struct {
	Fight   *Fight
	Current bool
}

type FightTracker struct {
	current      *Meter
	history      []*Fight
	pendingDeath *Death
	lastWallSeen time.Time
	historyLimit int
	activeTarget string
	hostiles     map[string]bool
	deadHostiles map[string]bool
}

func NewFightTracker() *FightTracker {
	return NewFightTrackerWithHistory(DefaultFightHistory)
}

func NewFightTrackerWithHistory(historyLimit int) *FightTracker {
	return &FightTracker{historyLimit: historyLimit}
}

func (t *FightTracker) AddDamage(event Event) {
	t.AddDamageWithIdle(event, DefaultIdleTimeout)
}

func (t *FightTracker) AddDamageWithIdle(event Event, idleTimeout time.Duration) {
	t.lastWallSeen = time.Now()
	if t.pendingDeath != nil {
		if event.Time.Sub(t.pendingDeath.Time) > deathGracePeriod {
			t.finalizePendingDeath()
		} else if !sameCombatant(event.Source, t.pendingDeath.Victim) && !sameCombatant(event.Target, t.pendingDeath.Victim) {
			t.finalizePendingDeath()
		}
	}
	if t.current != nil && t.pendingDeath == nil && idleTimeout > 0 && event.Time.Sub(t.current.Ended()) > idleTimeout {
		t.finalizeIdle("idle timeout")
	}
	if t.current == nil {
		t.current = NewMeter()
	}
	t.current.Add(event)
	t.trackHostiles(event)
}

func (t *FightTracker) AddDeath(death Death) {
	if t.current == nil || t.current.Events() == 0 {
		return
	}
	t.lastWallSeen = time.Now()
	if sameCombatant(death.Victim, "You") {
		t.current.ended = death.Time
		t.pendingDeath = &death
		t.finalizePendingDeath()
		return
	}

	victimKey := combatantKey(death.Victim)
	if t.hostiles[victimKey] {
		t.deadHostiles[victimKey] = true
	}
	if !sameCombatant(death.Victim, t.activeTarget) && !t.allHostilesDead() {
		return
	}

	t.current.ended = death.Time
	t.pendingDeath = &death
}

func (t *FightTracker) EndIdle(now time.Time, idleTimeout time.Duration) bool {
	if idleTimeout <= 0 || t.current == nil || t.current.Events() == 0 || t.lastWallSeen.IsZero() {
		return false
	}
	if t.pendingDeath != nil {
		if now.Sub(t.lastWallSeen) >= deathGracePeriod {
			t.finalizePendingDeath()
			return true
		}
		return false
	}
	if now.Sub(t.lastWallSeen) < idleTimeout {
		return false
	}
	t.finalizeIdle("idle timeout")
	return true
}

func (t *FightTracker) EndIdleAtLogTime(logTime time.Time, idleTimeout time.Duration) bool {
	if idleTimeout <= 0 || t.current == nil || t.current.Events() == 0 || logTime.IsZero() {
		return false
	}
	if t.pendingDeath != nil {
		if logTime.Sub(t.pendingDeath.Time) >= deathGracePeriod {
			t.finalizePendingDeath()
			return true
		}
		return false
	}
	if logTime.Sub(t.current.Ended()) < idleTimeout {
		return false
	}
	t.finalizeIdle("idle timeout")
	return true
}

func (t *FightTracker) DisplayFight() (*Fight, bool) {
	if t.pendingDeath != nil && t.current != nil && t.current.Events() > 0 {
		return &Fight{Meter: t.current, Death: *t.pendingDeath}, false
	}
	if t.current != nil && t.current.Events() > 0 {
		return &Fight{Meter: t.current}, true
	}
	if len(t.history) > 0 {
		return t.history[0], false
	}
	return nil, false
}

func (t *FightTracker) DisplaySections() []DisplaySection {
	sections := make([]DisplaySection, 0, t.displayCapacity())
	if t.pendingDeath != nil && t.current != nil && t.current.Events() > 0 {
		sections = append(sections, DisplaySection{
			Fight:   &Fight{Meter: t.current, Death: *t.pendingDeath},
			Current: false,
		})
	} else if t.current != nil && t.current.Events() > 0 {
		sections = append(sections, DisplaySection{
			Fight:   &Fight{Meter: t.current},
			Current: true,
		})
	}

	for _, fight := range t.history {
		if t.historyLimit > 0 && len(sections) >= t.historyLimit+1 {
			break
		}
		sections = append(sections, DisplaySection{Fight: fight})
	}

	return sections
}

func (t *FightTracker) displayCapacity() int {
	if t.historyLimit <= 0 {
		return len(t.history) + 1
	}
	return t.historyLimit + 1
}

func (t *FightTracker) finalizePendingDeath() {
	if t.pendingDeath == nil || t.current == nil || t.current.Events() == 0 {
		t.pendingDeath = nil
		return
	}
	t.history = append([]*Fight{{Meter: t.current, Death: *t.pendingDeath}}, t.history...)
	t.trimHistory()
	t.current = nil
	t.pendingDeath = nil
	t.resetEncounterState()
}

func (t *FightTracker) finalizeIdle(reason string) {
	if t.current == nil || t.current.Events() == 0 {
		return
	}
	t.history = append([]*Fight{{Meter: t.current, EndReason: reason}}, t.history...)
	t.trimHistory()
	t.current = nil
	t.pendingDeath = nil
	t.resetEncounterState()
}

func (t *FightTracker) trimHistory() {
	if t.historyLimit > 0 && len(t.history) > t.historyLimit {
		t.history = t.history[:t.historyLimit]
	}
}

func sameCombatant(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func combatantKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (t *FightTracker) trackHostiles(event Event) {
	if event.Amount <= 0 || event.Source == "" {
		return
	}
	if sameCombatant(event.Source, "You") && !sameCombatant(event.Target, "You") {
		t.addHostile(event.Target)
		if !event.Passive {
			t.activeTarget = event.Target
		}
	}
	if sameCombatant(event.Target, "You") && !sameCombatant(event.Source, "You") {
		t.addHostile(event.Source)
	}
}

func (t *FightTracker) addHostile(name string) {
	key := combatantKey(name)
	if key == "" {
		return
	}
	if t.hostiles == nil {
		t.hostiles = make(map[string]bool)
	}
	if t.deadHostiles == nil {
		t.deadHostiles = make(map[string]bool)
	}
	t.hostiles[key] = true
}

func (t *FightTracker) allHostilesDead() bool {
	if len(t.hostiles) == 0 {
		return false
	}
	for hostile := range t.hostiles {
		if !t.deadHostiles[hostile] {
			return false
		}
	}
	return true
}

func (t *FightTracker) resetEncounterState() {
	t.activeTarget = ""
	t.hostiles = nil
	t.deadHostiles = nil
}
