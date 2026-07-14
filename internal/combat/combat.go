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
	Time           time.Time
	Source         string
	Target         string
	Amount         int
	Kind           string
	Ability        string
	Critical       bool
	Passive        bool
	DamageOverTime bool
	Incidental     bool
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
	EngagedAt   time.Time
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
	return s.DPSForDuration(s.ActiveDuration())
}

func (s PlayerStats) DPSForDuration(duration time.Duration) float64 {
	seconds := duration.Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(s.Damage) / seconds
}

func (s PlayerStats) EngagedDPS(ended time.Time) (float64, bool) {
	if s.EngagedAt.IsZero() || ended.IsZero() || ended.Before(s.EngagedAt) {
		return 0, false
	}
	return s.DPSForDuration(ended.Sub(s.EngagedAt) + time.Second), true
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
	if event.Source == "You" && !event.Incidental && (!event.Passive || event.DamageOverTime) && stats.EngagedAt.IsZero() {
		stats.EngagedAt = event.Time
	}
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

func (m *Meter) resetEngagement() {
	if stats := m.players["You"]; stats != nil {
		stats.EngagedAt = time.Time{}
	}
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
	if event.DamageOverTime {
		return "DoTs"
	}
	if event.Ability != "" {
		return event.Ability
	}
	return "Melee"
}

type Fight struct {
	Mob       string
	Meter     *Meter
	Death     Death
	EndReason string
}

func (f *Fight) ActiveDuration() time.Duration {
	if f == nil || f.Meter == nil || f.Meter.Started().IsZero() || f.Meter.Ended().IsZero() {
		return 0
	}
	if f.Meter.Ended().Before(f.Meter.Started()) {
		return 0
	}
	return f.Meter.Ended().Sub(f.Meter.Started()) + time.Second
}

type DisplaySection struct {
	Fight   *Fight
	Current bool
}

type trackedMob struct {
	fight        *Fight
	pendingDeath *Death
	buffered     []Event
	lastWallSeen time.Time
	primarySeen  bool
}

type forgottenMob struct {
	fight    *Fight
	lastLog  time.Time
	lastWall time.Time
}

type FightTracker struct {
	active       map[string]*trackedMob
	history      []*Fight
	historyLimit int
	players      map[string]bool
	forgotten    map[string]*forgottenMob
}

func NewFightTracker() *FightTracker {
	return NewFightTrackerWithHistory(DefaultFightHistory)
}

func NewFightTrackerWithHistory(historyLimit int) *FightTracker {
	return &FightTracker{
		active:       make(map[string]*trackedMob),
		historyLimit: historyLimit,
		players:      map[string]bool{combatantKey("You"): true},
		forgotten:    make(map[string]*forgottenMob),
	}
}

func (t *FightTracker) AddDamage(event Event) {
	t.AddDamageWithIdle(event, DefaultIdleTimeout)
}

func (t *FightTracker) AddDamageWithIdle(event Event, idleTimeout time.Duration) {
	if event.Amount <= 0 || event.Source == "" || event.Target == "" {
		return
	}
	t.endAtLogTime(event.Time, idleTimeout)

	key, mob, mobEndpoint := t.mobForEvent(event)
	if key == "" {
		return
	}
	record := t.active[key]
	if record != nil && record.pendingDeath != nil {
		switch {
		case !event.Time.After(record.pendingDeath.Time):
			t.addEvent(record, event, mobEndpoint)
			return
		case event.DamageOverTime:
			record.buffered = append(record.buffered, event)
			return
		default:
			buffered := append([]Event(nil), record.buffered...)
			record.buffered = nil
			t.finalize(record)
			record = &trackedMob{fight: &Fight{Mob: mob, Meter: NewMeter()}}
			t.active[key] = record
			for _, bufferedEvent := range buffered {
				t.addEvent(record, bufferedEvent, mobEndpointFor(bufferedEvent, record.fight.Mob))
			}
		}
	}
	if record == nil {
		if forgotten := t.forgotten[key]; forgotten != nil {
			if event.DamageOverTime {
				forgotten.fight.Meter.Add(event)
				forgotten.lastLog = event.Time
				forgotten.lastWall = time.Now()
				return
			}
			delete(t.forgotten, key)
		}
		record = &trackedMob{fight: &Fight{Mob: mob, Meter: NewMeter()}}
		t.active[key] = record
	}
	t.addEvent(record, event, mobEndpoint)
}

func (t *FightTracker) ForgetEnemies(timestamp time.Time) {
	if timestamp.IsZero() {
		return
	}
	now := time.Now()
	for _, record := range t.sortedActive() {
		if record.fight.Death.Victim == "" {
			record.fight.EndReason = "enemies forgot you"
		}
		record.fight.Meter.ended = timestamp
		key := combatantKey(record.fight.Mob)
		t.finalize(record)
		t.forgotten[key] = &forgottenMob{
			fight:    record.fight,
			lastLog:  timestamp,
			lastWall: now,
		}
	}
}

func (t *FightTracker) addEvent(record *trackedMob, event Event, mobEndpoint string) {
	record.lastWallSeen = time.Now()
	record.fight.Meter.Add(event)
	if sameCombatant(mobEndpoint, record.fight.Mob) {
		record.primarySeen = true
	}
}

func (t *FightTracker) AddDeath(death Death) {
	if len(t.active) == 0 {
		return
	}
	if sameCombatant(death.Victim, "You") {
		for _, record := range t.sortedActive() {
			record.fight.Meter.ended = death.Time
			record.fight.Death = death
			t.finalize(record)
		}
		return
	}

	key, _, aliased := t.mobIdentity(death.Victim)
	record := t.active[key]
	if record == nil {
		return
	}
	if aliased && record.primarySeen {
		return
	}
	for activeKey, activeRecord := range t.active {
		if activeKey != key {
			activeRecord.fight.Meter.resetEngagement()
		}
	}
	if record.pendingDeath != nil {
		buffered := append([]Event(nil), record.buffered...)
		record.buffered = nil
		mob := record.fight.Mob
		t.finalize(record)
		if len(buffered) == 0 {
			return
		}
		record = &trackedMob{fight: &Fight{Mob: mob, Meter: NewMeter()}}
		t.active[key] = record
		for _, event := range buffered {
			t.addEvent(record, event, mobEndpointFor(event, mob))
		}
	}
	record.lastWallSeen = time.Now()
	record.fight.Meter.ended = death.Time
	record.fight.Death = death
	record.pendingDeath = &death
}

func (t *FightTracker) EndIdle(now time.Time, idleTimeout time.Duration) bool {
	t.expireForgottenAtWall(now)
	changed := false
	for _, record := range t.sortedActive() {
		if record.pendingDeath != nil {
			if now.Sub(record.lastWallSeen) >= deathGracePeriod {
				t.finalize(record)
				changed = true
			}
			continue
		}
		if idleTimeout > 0 && !record.lastWallSeen.IsZero() && now.Sub(record.lastWallSeen) >= idleTimeout {
			record.fight.EndReason = "idle timeout"
			t.finalize(record)
			changed = true
		}
	}
	return changed
}

func (t *FightTracker) EndIdleAtLogTime(logTime time.Time, idleTimeout time.Duration) bool {
	if logTime.IsZero() {
		return false
	}
	return t.endAtLogTime(logTime, idleTimeout)
}

func (t *FightTracker) DisplayFight() (*Fight, bool) {
	sections := t.DisplaySections()
	if len(sections) == 0 {
		return nil, false
	}
	return sections[0].Fight, sections[0].Current
}

func (t *FightTracker) DisplaySections() []DisplaySection {
	active := t.sortedActive()
	sections := make([]DisplaySection, 0, len(active)+len(t.history))
	for _, record := range active {
		sections = append(sections, DisplaySection{
			Fight:   record.fight,
			Current: record.pendingDeath == nil,
		})
	}

	for _, fight := range t.history {
		sections = append(sections, DisplaySection{Fight: fight})
	}

	return sections
}

func (t *FightTracker) finalize(record *trackedMob) {
	if record == nil || record.fight == nil || record.fight.Meter.Events() == 0 {
		return
	}
	for _, event := range record.buffered {
		record.fight.Meter.Add(event)
	}
	record.buffered = nil
	delete(t.active, combatantKey(record.fight.Mob))
	t.history = append([]*Fight{record.fight}, t.history...)
	t.trimHistory()
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

func (t *FightTracker) endAtLogTime(logTime time.Time, idleTimeout time.Duration) bool {
	t.expireForgottenAtLogTime(logTime)
	changed := false
	for _, record := range t.sortedActive() {
		if record.pendingDeath != nil {
			if logTime.Sub(record.pendingDeath.Time) >= deathGracePeriod {
				t.finalize(record)
				changed = true
			}
			continue
		}
		if idleTimeout > 0 && logTime.Sub(record.fight.Meter.Ended()) >= idleTimeout {
			record.fight.EndReason = "idle timeout"
			t.finalize(record)
			changed = true
		}
	}
	return changed
}

func (t *FightTracker) expireForgottenAtLogTime(logTime time.Time) {
	for key, record := range t.forgotten {
		if logTime.Sub(record.lastLog) >= deathGracePeriod {
			delete(t.forgotten, key)
		}
	}
}

func (t *FightTracker) expireForgottenAtWall(now time.Time) {
	for key, record := range t.forgotten {
		if now.Sub(record.lastWall) >= deathGracePeriod {
			delete(t.forgotten, key)
		}
	}
}

func mobEndpointFor(event Event, mob string) string {
	if sameCombatant(event.Source, mob) {
		return event.Source
	}
	if sameCombatant(event.Target, mob) {
		return event.Target
	}
	return ""
}

func (t *FightTracker) sortedActive() []*trackedMob {
	records := make([]*trackedMob, 0, len(t.active))
	for _, record := range t.active {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		left := records[i].fight.Meter.Started()
		right := records[j].fight.Meter.Started()
		if left.Equal(right) {
			return records[i].fight.Mob < records[j].fight.Mob
		}
		return left.Before(right)
	})
	return records
}

func (t *FightTracker) mobForEvent(event Event) (string, string, string) {
	sourceKey, _, _ := t.mobIdentity(event.Source)
	targetKey, _, _ := t.mobIdentity(event.Target)
	sourceIsMob := t.active[sourceKey] != nil
	targetIsMob := t.active[targetKey] != nil
	sourceIsPlayer := t.players[combatantKey(event.Source)]
	targetIsPlayer := t.players[combatantKey(event.Target)]

	mobEndpoint := event.Target
	switch {
	case sameCombatant(event.Target, "You"):
		mobEndpoint = event.Source
	case sameCombatant(event.Source, "You"):
		mobEndpoint = event.Target
	case sourceIsMob && !targetIsMob:
		mobEndpoint = event.Source
	case targetIsMob:
		mobEndpoint = event.Target
	case sourceIsPlayer && !targetIsPlayer:
		mobEndpoint = event.Target
	case targetIsPlayer:
		mobEndpoint = event.Source
	}

	key, mob, _ := t.mobIdentity(mobEndpoint)
	if sameCombatant(mobEndpoint, event.Source) {
		if !targetIsMob {
			t.players[combatantKey(event.Target)] = true
		}
	} else if !sourceIsMob {
		t.players[combatantKey(event.Source)] = true
	}
	return key, mob, mobEndpoint
}

func (t *FightTracker) mobIdentity(name string) (string, string, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || sameCombatant(trimmed, "You") {
		return "", "", false
	}
	if owner, ok := petSuffixOwner(trimmed); ok {
		return combatantKey(owner), owner, true
	}
	if owner, _, ok := possessiveOwner(trimmed); ok && t.active[combatantKey(owner)] != nil {
		return combatantKey(owner), owner, true
	}
	return combatantKey(trimmed), trimmed, false
}

func petSuffixOwner(name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	if len(trimmed) <= len(" pet") || !strings.EqualFold(trimmed[len(trimmed)-len(" pet"):], " pet") {
		return "", false
	}
	return strings.TrimSpace(trimmed[:len(trimmed)-len(" pet")]), true
}
