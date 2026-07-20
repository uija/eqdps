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

func TestOverlayPrefersCurrentFightMostRecentlyDamagedIntentionallyByYou(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	overlay := combatOverlay{fights: []fakeFightSection{
		{name: "newer incidental fight", current: true, started: now.Add(10 * time.Second)},
		{name: "your target", current: true, started: now, lastYouIntentional: now.Add(5 * time.Second)},
		{name: "your previous target", current: true, started: now.Add(2 * time.Second), lastYouIntentional: now.Add(time.Second)},
	}}

	if got := overlay.displayFight(); got == nil || got.name != "your target" {
		t.Fatalf("expected latest intentional target, got %#v", got)
	}
}

func TestWaylandSessionDetection(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "")
	if !isWaylandSession() {
		t.Fatal("expected XDG Wayland session to be detected")
	}

	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	if !isWaylandSession() {
		t.Fatal("expected WAYLAND_DISPLAY to be detected")
	}

	t.Setenv("WAYLAND_DISPLAY", "")
	if isWaylandSession() {
		t.Fatal("did not expect X11 session to be detected as Wayland")
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
