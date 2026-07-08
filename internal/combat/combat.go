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
	players := make([]PlayerStats, 0, len(m.players))
	for _, stats := range m.players {
		player := *stats
		player.DamageTypes = make(map[string]int, len(stats.DamageTypes))
		for name, amount := range stats.DamageTypes {
			player.DamageTypes[name] = amount
		}
		players = append(players, player)
	}

	sort.Slice(players, func(i, j int) bool {
		if players[i].FirstSeen.Equal(players[j].FirstSeen) {
			return players[i].Name < players[j].Name
		}
		return players[i].FirstSeen.Before(players[j].FirstSeen)
	})

	return players
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
}

func (t *FightTracker) AddDeath(death Death) {
	if t.current == nil || t.current.Events() == 0 {
		return
	}
	t.lastWallSeen = time.Now()
	t.current.ended = death.Time
	t.pendingDeath = &death
	if sameCombatant(death.Victim, "You") {
		t.finalizePendingDeath()
	}
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
}

func (t *FightTracker) finalizeIdle(reason string) {
	if t.current == nil || t.current.Events() == 0 {
		return
	}
	t.history = append([]*Fight{{Meter: t.current, EndReason: reason}}, t.history...)
	t.trimHistory()
	t.current = nil
	t.pendingDeath = nil
}

func (t *FightTracker) trimHistory() {
	if t.historyLimit > 0 && len(t.history) > t.historyLimit {
		t.history = t.history[:t.historyLimit]
	}
}

func sameCombatant(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}
