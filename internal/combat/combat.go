package combat

import (
	"sort"
	"strings"
	"time"
)

const deathGracePeriod = 8 * time.Second
const castMatchWindow = 30 * time.Second
const DefaultIdleTimeout = 15 * time.Second
const DefaultFightHistory = 0

type Event struct {
	Time           time.Time
	Source         string
	Target         string
	Amount         int
	Attack         string
	Ability        string
	Critical       bool
	Passive        bool
	DamageOverTime bool
	Incidental     bool
	DirectCast     bool
}

type Cast struct {
	Time    time.Time
	Source  string
	Ability string
}

type Death struct {
	Time   time.Time
	Victim string
	Killer string
}

type PlayerStats struct {
	Name      string
	Damage    int
	Hits      int
	Crits     int
	MinHit    int
	MaxHit    int
	FirstSeen time.Time
	LastSeen  time.Time
	Breakdown map[string]*BreakdownStats
	EngagedAt time.Time
}

type BreakdownStats struct {
	Name      string
	Damage    int
	Hits      int
	Crits     int
	MinHit    int
	MaxHit    int
	FirstSeen time.Time
	LastSeen  time.Time
	Children  map[string]*BreakdownStats
}

func (s BreakdownStats) ActiveDuration() time.Duration {
	return activeDuration(s.FirstSeen, s.LastSeen)
}

func (s BreakdownStats) DPS() float64 {
	seconds := s.ActiveDuration().Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(s.Damage) / seconds
}

func (s BreakdownStats) SortedChildren() []BreakdownStats {
	return sortedBreakdown(s.Children)
}

func (s PlayerStats) ActiveDuration() time.Duration {
	return activeDuration(s.FirstSeen, s.LastSeen)
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
			Name:      event.Source,
			FirstSeen: event.Time,
			Breakdown: make(map[string]*BreakdownStats),
		}
		m.players[event.Source] = stats
	}

	stats.Damage += event.Amount
	category, detail := damageBreakdownNames(event)
	addBreakdownEvent(stats.Breakdown, category, event)
	if detail != "" {
		addBreakdownEvent(stats.Breakdown[category].Children, detail, event)
	}
	stats.Hits++
	if stats.MinHit == 0 || event.Amount < stats.MinHit {
		stats.MinHit = event.Amount
	}
	if event.Amount > stats.MaxHit {
		stats.MaxHit = event.Amount
	}
	if event.Critical {
		stats.Crits++
	}
	if stats.FirstSeen.IsZero() || event.Time.Before(stats.FirstSeen) {
		stats.FirstSeen = event.Time
	}
	if stats.LastSeen.IsZero() || event.Time.After(stats.LastSeen) {
		stats.LastSeen = event.Time
	}
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
	copied.Breakdown = copyBreakdown(stats.Breakdown)
	return &copied
}

func mergePetStats(owner, pet *PlayerStats, petName string) {
	owner.Damage += pet.Damage
	owner.Hits += pet.Hits
	owner.Crits += pet.Crits
	if owner.MinHit == 0 || (pet.MinHit > 0 && pet.MinHit < owner.MinHit) {
		owner.MinHit = pet.MinHit
	}
	if pet.MaxHit > owner.MaxHit {
		owner.MaxHit = pet.MaxHit
	}
	if owner.FirstSeen.IsZero() || (!pet.FirstSeen.IsZero() && pet.FirstSeen.Before(owner.FirstSeen)) {
		owner.FirstSeen = pet.FirstSeen
	}
	if owner.LastSeen.IsZero() || pet.LastSeen.After(owner.LastSeen) {
		owner.LastSeen = pet.LastSeen
	}
	if owner.Breakdown == nil {
		owner.Breakdown = make(map[string]*BreakdownStats)
	}
	petStats := &BreakdownStats{
		Name:      "Pet: " + petName,
		Damage:    pet.Damage,
		Hits:      pet.Hits,
		Crits:     pet.Crits,
		FirstSeen: pet.FirstSeen,
		LastSeen:  pet.LastSeen,
		MinHit:    pet.MinHit,
		MaxHit:    pet.MaxHit,
		Children:  copyBreakdown(pet.Breakdown),
	}
	owner.Breakdown[petStats.Name] = petStats
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

func (s PlayerStats) DamageBreakdown() []BreakdownStats {
	return sortedBreakdown(s.Breakdown)
}

func sortedBreakdown(entries map[string]*BreakdownStats) []BreakdownStats {
	breakdown := make([]BreakdownStats, 0, len(entries))
	for _, entry := range entries {
		breakdown = append(breakdown, *entry)
	}
	sort.Slice(breakdown, func(i, j int) bool {
		if breakdown[i].Damage == breakdown[j].Damage {
			return breakdown[i].Name < breakdown[j].Name
		}
		return breakdown[i].Damage > breakdown[j].Damage
	})
	return breakdown
}

func damageBreakdownNames(event Event) (string, string) {
	if event.DamageOverTime {
		return "DoTs", fallbackName(event.Ability, "Unknown DoT")
	}
	if event.Passive {
		return "Damage Shield", fallbackName(event.Ability, "Unknown Shield")
	}
	if event.Ability != "" {
		if event.DirectCast {
			return "Magic", event.Ability
		}
		return "Procs", event.Ability
	}
	return "Melee", meleeName(event.Attack)
}

func addBreakdownEvent(entries map[string]*BreakdownStats, name string, event Event) {
	entry := entries[name]
	if entry == nil {
		entry = &BreakdownStats{Name: name, Children: make(map[string]*BreakdownStats)}
		entries[name] = entry
	}
	entry.Damage += event.Amount
	entry.Hits++
	if event.Critical {
		entry.Crits++
	}
	mergeMinMax(entry, event.Amount)
	if entry.FirstSeen.IsZero() || event.Time.Before(entry.FirstSeen) {
		entry.FirstSeen = event.Time
	}
	if entry.LastSeen.IsZero() || event.Time.After(entry.LastSeen) {
		entry.LastSeen = event.Time
	}
}

func mergeMinMax(stats *BreakdownStats, amount int) {
	if amount <= 0 {
		return
	}
	if stats.MinHit == 0 || amount < stats.MinHit {
		stats.MinHit = amount
	}
	if amount > stats.MaxHit {
		stats.MaxHit = amount
	}
}

func copyBreakdown(entries map[string]*BreakdownStats) map[string]*BreakdownStats {
	copied := make(map[string]*BreakdownStats, len(entries))
	for name, entry := range entries {
		entryCopy := *entry
		entryCopy.Children = copyBreakdown(entry.Children)
		copied[name] = &entryCopy
	}
	return copied
}

func fallbackName(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func meleeName(attack string) string {
	switch strings.ToLower(strings.TrimSpace(attack)) {
	case "backstab", "backstabs":
		return "Backstabs"
	case "bash", "bashes":
		return "Bashes"
	case "bite", "bites":
		return "Bites"
	case "cleave", "cleaves":
		return "Cleaves"
	case "claw", "claws":
		return "Claws"
	case "crush", "crushes":
		return "Crushes"
	case "frenzy on", "frenzies on":
		return "Frenzies"
	case "hit", "hits":
		return "Hits"
	case "kick", "kicks":
		return "Kicks"
	case "maul", "mauls":
		return "Mauls"
	case "pierce", "pierces":
		return "Pierces"
	case "punch", "punches":
		return "Punches"
	case "reave", "reaves":
		return "Reaves"
	case "shoot", "shoots":
		return "Shots"
	case "slash", "slashes":
		return "Slashes"
	case "slice", "slices":
		return "Slices"
	case "smash", "smashes":
		return "Smashes"
	case "smite", "smites":
		return "Smites"
	case "strike", "strikes":
		return "Strikes"
	default:
		return "Hits"
	}
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
	return activeDuration(f.Meter.Started(), f.Meter.Ended())
}

func activeDuration(first, last time.Time) time.Duration {
	if first.IsZero() || last.IsZero() || last.Before(first) {
		return 0
	}
	return last.Sub(first) + time.Second
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
	pendingCasts map[string]pendingCast
}

type pendingCast struct {
	started   time.Time
	matchedAt time.Time
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
		pendingCasts: make(map[string]pendingCast),
	}
}

func (t *FightTracker) AddCast(cast Cast) {
	if cast.Time.IsZero() || cast.Source == "" || cast.Ability == "" {
		return
	}
	t.expireCasts(cast.Time)
	t.pendingCasts[castKey(cast.Source, cast.Ability)] = pendingCast{started: cast.Time}
}

func (t *FightTracker) AddDamage(event Event) {
	t.AddDamageWithIdle(event, DefaultIdleTimeout)
}

func (t *FightTracker) AddDamageWithIdle(event Event, idleTimeout time.Duration) {
	if event.Amount <= 0 || event.Source == "" || event.Target == "" {
		return
	}
	t.expireCasts(event.Time)
	t.classifyDirectCast(&event)
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

func (t *FightTracker) classifyDirectCast(event *Event) {
	if event == nil || event.Ability == "" || event.DamageOverTime || event.Passive {
		return
	}
	key := castKey(event.Source, event.Ability)
	pending, ok := t.pendingCasts[key]
	if !ok {
		return
	}
	if event.Time.Before(pending.started) || event.Time.Sub(pending.started) > castMatchWindow || (!pending.matchedAt.IsZero() && !event.Time.Equal(pending.matchedAt)) {
		delete(t.pendingCasts, key)
		return
	}
	event.DirectCast = true
	if pending.matchedAt.IsZero() {
		pending.matchedAt = event.Time
		t.pendingCasts[key] = pending
	}
}

func castKey(source, ability string) string {
	return combatantKey(source) + "\x00" + strings.ToLower(strings.TrimSpace(ability))
}

func (t *FightTracker) expireCasts(timestamp time.Time) {
	for key, pending := range t.pendingCasts {
		if timestamp.After(pending.started) && timestamp.Sub(pending.started) > castMatchWindow {
			delete(t.pendingCasts, key)
		}
	}
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
