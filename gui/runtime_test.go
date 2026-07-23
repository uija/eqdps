package main

import (
	"testing"
	"time"

	"github.com/uija/eqdps/internal/combat"
)

func TestCombatUpdateCoalescingPreservesReplayCompletion(t *testing.T) {
	shell := shell{combatUpdates: make(chan combatUpdate, 1)}
	shell.combatUpdates <- combatUpdate{loadDone: true, status: "history loaded"}
	shell.sendCombatUpdate(combatUpdate{fights: []fakeFightSection{{name: "first live fight"}}, status: "live"})

	update := <-shell.combatUpdates
	if !update.loadDone {
		t.Fatal("live snapshot dropped replay completion")
	}
	if len(update.fights) != 1 || update.fights[0].name != "first live fight" || update.status != "live" {
		t.Fatalf("unexpected merged update: %#v", update)
	}
}

func TestFormatKillTime(t *testing.T) {
	when := time.Date(2026, time.July, 21, 9, 17, 42, 0, time.Local)
	if got := formatKillTime(combat.Death{Time: when, Victim: "a goblin"}); got != "Killed 2026-07-21 09:17" {
		t.Fatalf("unexpected kill time: %q", got)
	}
	if got := formatKillTime(combat.Death{Time: when, Victim: "You"}); got != "" {
		t.Fatalf("player death should not be shown as a mob kill: %q", got)
	}
	if got := formatKillTime(combat.Death{}); got != "" {
		t.Fatalf("empty death should not have a kill time: %q", got)
	}
}

func TestMergeCombatUpdatesRetainsLatestDataAndMissingFields(t *testing.T) {
	xp := combatUpdate{status: "pending", state: "loading", fights: []fakeFightSection{{name: "pending"}}}
	merged := mergeCombatUpdates(xp, combatUpdate{status: "latest"})
	if merged.status != "latest" || merged.state != "loading" || len(merged.fights) != 1 {
		t.Fatalf("unexpected merged update: %#v", merged)
	}
}
