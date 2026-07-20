package main

import (
	"testing"
	"time"
)

func TestOverlayDisplaysLatestCompletedFightBetweenFights(t *testing.T) {
	overlay := combatOverlay{fights: []fakeFightSection{
		{name: "latest completed"},
		{name: "older completed"},
	}}

	if got := overlay.displayFight(); got == nil || got.name != "latest completed" {
		t.Fatalf("expected latest completed fight, got %#v", got)
	}
}

func TestOverlayPrefersNewestOfConcurrentCurrentFights(t *testing.T) {
	started := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	overlay := combatOverlay{fights: []fakeFightSection{
		{name: "older current", current: true, started: started},
		{name: "newer current", current: true, started: started.Add(time.Second)},
	}}

	if got := overlay.displayFight(); got == nil || got.name != "newer current" {
		t.Fatalf("expected newest current fight, got %#v", got)
	}
}

func TestOverlayPrefersCurrentFightOverHistory(t *testing.T) {
	overlay := combatOverlay{fights: []fakeFightSection{
		{name: "latest completed"},
		{name: "current", current: true},
	}}

	if got := overlay.displayFight(); got == nil || got.name != "current" {
		t.Fatalf("expected current fight, got %#v", got)
	}
}
