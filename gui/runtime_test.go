package main

import "testing"

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

func TestMergeCombatUpdatesRetainsLatestDataAndMissingFields(t *testing.T) {
	xp := combatUpdate{status: "pending", state: "loading", fights: []fakeFightSection{{name: "pending"}}}
	merged := mergeCombatUpdates(xp, combatUpdate{status: "latest"})
	if merged.status != "latest" || merged.state != "loading" || len(merged.fights) != 1 {
		t.Fatalf("unexpected merged update: %#v", merged)
	}
}
