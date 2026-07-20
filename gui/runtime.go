package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/uija/eqdps/internal/combat"
	"github.com/uija/eqdps/internal/engine"
	"github.com/uija/eqdps/internal/xp"
)

type combatUpdate struct {
	fights   []fakeFightSection
	status   string
	progress *engine.ReplayProgress
	loadDone bool
	xp       *xp.Snapshot
	state    string
}

func (s *shell) loadLog(path string, back time.Duration) {
	if s.logCancel != nil {
		close(s.logCancel)
	}
	cancel := make(chan struct{})
	s.logCancel = cancel
	s.fights = nil
	s.allFights = nil
	s.xpSnapshot = xp.Snapshot{}
	s.parserState = "loading"
	s.loading = back != 0
	s.loadBytes, s.loadTotal, s.loadLines = 0, 0, 0
	s.statusText = filepathBase(path) + " · loading " + historyStatus(back) + "…"
	go func() {
		info, err := os.Stat(path)
		if err != nil {
			s.sendCombatUpdate(combatUpdate{status: err.Error(), loadDone: true, state: "error"})
			return
		}
		limit := info.Size()
		if back != 0 {
			progress := engine.ReplayProgress{Total: limit}
			s.sendCombatUpdate(combatUpdate{progress: &progress})
		}
		tracker := combat.NewFightTracker()
		xpSession := xp.NewSession()
		if back != 0 {
			tracker, xpSession, err = engine.ReplayWithProgress(path, combat.DefaultIdleTimeout, back, time.Time{}, combat.DefaultFightHistory, limit, func(progress engine.ReplayProgress) {
				s.sendCombatUpdate(combatUpdate{progress: &progress})
			}, cancel)
			if err != nil {
				if !errors.Is(err, engine.ErrReplayCancelled) {
					s.sendCombatUpdate(combatUpdate{status: err.Error(), loadDone: true, state: "error"})
				} else {
					s.sendCombatUpdate(combatUpdate{loadDone: true})
				}
				return
			}
		}
		xpSnapshot := xpSession.SnapshotAtLatestLog()
		s.sendCombatUpdate(combatUpdate{fights: snapshotFights(tracker), status: filepathBase(path) + " · " + historyStatus(back), loadDone: true, xp: &xpSnapshot, state: "live"})
		err = engine.Follow(path, limit, cancel, func(line string, _ int64) {
			engine.ProcessLine(line, tracker, xpSession, combat.DefaultIdleTimeout)
			xpSnapshot := xpSession.SnapshotLive(time.Now())
			s.sendCombatUpdate(combatUpdate{fights: snapshotFights(tracker), status: filepathBase(path) + " · live", xp: &xpSnapshot, state: "live"})
		})
		if err != nil {
			s.sendCombatUpdate(combatUpdate{status: err.Error(), state: "error"})
		}
	}()
}

func (s *shell) sendCombatUpdate(update combatUpdate) {
	select {
	case s.combatUpdates <- update:
	default:
		select {
		case <-s.combatUpdates:
		default:
		}
		s.combatUpdates <- update
	}
	s.window.Invalidate()
}

func snapshotFights(tracker *combat.FightTracker) []fakeFightSection {
	sections := tracker.DisplaySections()
	result := make([]fakeFightSection, 0, len(sections))
	for _, section := range sections {
		fight := section.Fight
		status := fight.EndReason
		if section.Current {
			status = "current fight"
		} else if fight.Death.Killer != "" {
			status = "slain by " + fight.Death.Killer
		}
		entry := fakeFightSection{name: fight.Mob, status: status, duration: formatRuntimeDuration(fight.ActiveDuration()), current: section.Current, started: fight.Meter.Started()}
		for _, player := range fight.Meter.Players() {
			sdps, _ := player.EngagedDPS(fight.Meter.Ended())
			combatant := fakeCombatant{name: player.Name, damage: player.Damage, dps: int(player.DPS() + .5), sdps: int(sdps + .5), hits: player.Hits, crits: player.Crits, active: formatRuntimeDuration(player.ActiveDuration()), accent: player.Name == "You"}
			for _, detail := range sortedPlayerBreakdown(player.Breakdown) {
				combatant.details = append(combatant.details, snapshotBreakdown(detail))
			}
			entry.combatants = append(entry.combatants, combatant)
		}
		result = append(result, entry)
	}
	return result
}

func sortedPlayerBreakdown(values map[string]*combat.BreakdownStats) []combat.BreakdownStats {
	result := make([]combat.BreakdownStats, 0, len(values))
	for _, value := range values {
		result = append(result, *value)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Damage != result[right].Damage {
			return result[left].Damage > result[right].Damage
		}
		return result[left].Name < result[right].Name
	})
	return result
}

func snapshotBreakdown(detail combat.BreakdownStats) fakeBreakdown {
	entry := fakeBreakdown{name: detail.Name, damage: detail.Damage, dps: int(detail.DPS() + .5), hits: detail.Hits, crits: detail.Crits, active: formatRuntimeDuration(detail.ActiveDuration())}
	for _, child := range detail.SortedChildren() {
		entry.children = append(entry.children, snapshotBreakdown(child))
	}
	return entry
}

func formatRuntimeDuration(duration time.Duration) string {
	seconds := int(duration.Round(time.Second).Seconds())
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func filepathBase(path string) string {
	return filepath.Base(path)
}
