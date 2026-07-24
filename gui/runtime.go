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
	"github.com/uija/eqdps/internal/eqlog"
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
	s.loadingTitle = "Loading combat history…"
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
			idleTimeout := time.Duration(s.combatIdleNanos.Load())
			tracker, xpSession, err = engine.ReplayWithProgress(path, idleTimeout, back, time.Time{}, combat.DefaultFightHistory, limit, func(progress engine.ReplayProgress) {
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
		err = engine.FollowWithPoll(path, limit, cancel, func(line string, endOffset int64) {
			idleTimeout := time.Duration(s.combatIdleNanos.Load())
			engine.ProcessLine(line, tracker, xpSession, idleTimeout)
			if s.eqldb != nil {
				if record, ok := eqlog.ParseRecord(line); ok {
					s.eqldb.Observe(record)
				}
			}
			s.processSkyLine(path, line, endOffset)
			xpSnapshot := xpSession.SnapshotLive(time.Now())
			s.sendCombatUpdate(combatUpdate{fights: snapshotFights(tracker), status: filepathBase(path) + " · live", xp: &xpSnapshot, state: "live"})
		}, func(now time.Time) {
			idleTimeout := time.Duration(s.combatIdleNanos.Load())
			if tracker.EndIdle(now, idleTimeout) {
				s.sendCombatUpdate(combatUpdate{fights: snapshotFights(tracker), status: filepathBase(path) + " · live", state: "live"})
			}
		})
		if err != nil {
			s.sendCombatUpdate(combatUpdate{status: err.Error(), state: "error"})
		}
	}()
}

func (s *shell) sendCombatUpdate(update combatUpdate) {
	if update.fights != nil {
		s.pushOverlay(update.fights)
	}
	s.combatSendMu.Lock()
	defer s.combatSendMu.Unlock()
	select {
	case s.combatUpdates <- update:
	default:
		select {
		case pending := <-s.combatUpdates:
			update = mergeCombatUpdates(pending, update)
		default:
		}
		s.combatUpdates <- update
	}
	if s.window != nil {
		s.window.Invalidate()
	}
}

func mergeCombatUpdates(pending, next combatUpdate) combatUpdate {
	// Completion is an edge-triggered state transition. It must survive a
	// following live snapshot or the loading modal can remain open forever.
	next.loadDone = next.loadDone || pending.loadDone
	if next.fights == nil {
		next.fights = pending.fights
	}
	if next.status == "" {
		next.status = pending.status
	}
	if next.progress == nil && !next.loadDone {
		next.progress = pending.progress
	}
	if next.xp == nil {
		next.xp = pending.xp
	}
	if next.state == "" {
		next.state = pending.state
	}
	return next
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
		entry := fakeFightSection{name: fight.Mob, status: status, killedAt: formatKillTime(fight.Death), duration: formatRuntimeDuration(fight.ActiveDuration()), current: section.Current, started: fight.Meter.Started(), lastYouIntentionalOrder: fight.LastYouIntentionalOrder}
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

func formatKillTime(death combat.Death) string {
	if death.Time.IsZero() || death.Victim == "You" {
		return ""
	}
	return "Killed " + death.Time.Format("2006-01-02 15:04")
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
