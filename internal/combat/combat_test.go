package combat

import (
	"testing"
	"time"
)

func TestFightTrackerShowsCurrentThenLastFight(t *testing.T) {
	tracker := NewFightTracker()
	now := time.Date(2026, 7, 2, 5, 19, 3, 0, time.UTC)

	tracker.AddDamage(Event{
		Time:   now,
		Source: "You",
		Target: "a lizardman scout",
		Amount: 10,
	})

	fight, current := tracker.DisplayFight()
	if fight == nil || !current {
		t.Fatalf("expected current fight, got fight=%#v current=%v", fight, current)
	}
	if fight.Meter.Events() != 1 {
		t.Fatalf("expected one event, got %d", fight.Meter.Events())
	}

	tracker.AddDeath(Death{
		Time:   now.Add(5 * time.Second),
		Victim: "a lizardman scout",
		Killer: "You",
	})

	fight, current = tracker.DisplayFight()
	if fight == nil || current {
		t.Fatalf("expected last fight, got fight=%#v current=%v", fight, current)
	}
	if fight.Death.Victim != "a lizardman scout" {
		t.Fatalf("unexpected death: %#v", fight.Death)
	}
}

func TestMeterPlayersStayInFirstSeenOrder(t *testing.T) {
	meter := NewMeter()
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	meter.Add(Event{Time: now, Source: "You", Target: "mob", Amount: 10})
	meter.Add(Event{Time: now.Add(time.Second), Source: "A rock golem", Target: "YOU", Amount: 1000})

	players := meter.Players()
	if len(players) != 2 {
		t.Fatalf("expected two combatants, got %d", len(players))
	}
	if players[0].Name != "You" || players[1].Name != "A rock golem" {
		t.Fatalf("unexpected order: %#v", players)
	}
}

func TestMeterTracksDamageBreakdown(t *testing.T) {
	meter := NewMeter()
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	meter.Add(Event{Time: now, Source: "You", Target: "mob", Amount: 10})
	meter.Add(Event{Time: now.Add(time.Second), Source: "You", Target: "mob", Amount: 40, Ability: "Spell1"})
	meter.Add(Event{Time: now.Add(2 * time.Second), Source: "You", Target: "mob", Amount: 90, Ability: "Spell2"})
	meter.Add(Event{Time: now.Add(3 * time.Second), Source: "You", Target: "mob", Amount: 90})

	players := meter.Players()
	if len(players) != 1 {
		t.Fatalf("expected one combatant, got %d", len(players))
	}

	breakdown := players[0].DamageBreakdown()
	if len(breakdown) != 3 {
		t.Fatalf("expected 3 damage types, got %#v", breakdown)
	}
	expected := map[string]int{"Melee": 100, "Spell1": 40, "Spell2": 90}
	for _, entry := range breakdown {
		if expected[entry.Name] != entry.Damage {
			t.Fatalf("unexpected breakdown entry: %#v", entry)
		}
		delete(expected, entry.Name)
	}
	if len(expected) != 0 {
		t.Fatalf("missing breakdown entries: %#v", expected)
	}
}

func TestMeterKeepsPossessiveMobAsOwnCombatant(t *testing.T) {
	meter := NewMeter()
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	meter.Add(Event{Time: now, Source: "Innoruuk`s Chosen", Target: "YOU", Amount: 37})

	players := meter.Players()
	if len(players) != 1 {
		t.Fatalf("expected one combatant, got %#v", players)
	}
	if players[0].Name != "Innoruuk`s Chosen" {
		t.Fatalf("unexpected combatant: %#v", players[0])
	}
}

func TestMeterMergesPossessivePetWhenOwnerIsInFight(t *testing.T) {
	meter := NewMeter()
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	meter.Add(Event{Time: now, Source: "Sobatin`s warder", Target: "an orc raider", Amount: 4})
	meter.Add(Event{Time: now.Add(time.Second), Source: "Sobatin", Target: "an orc raider", Amount: 11, Ability: "Burst of Fire"})

	players := meter.Players()
	if len(players) != 1 {
		t.Fatalf("expected merged owner combatant, got %#v", players)
	}
	if players[0].Name != "Sobatin" || players[0].Damage != 15 {
		t.Fatalf("unexpected merged stats: %#v", players[0])
	}
	if players[0].DamageTypes["Pet: warder"] != 4 {
		t.Fatalf("expected pet damage bucket, got %#v", players[0].DamageTypes)
	}
}

func TestMeterMergesApostrophePossessivePetWhenOwnerIsInFight(t *testing.T) {
	meter := NewMeter()
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	meter.Add(Event{Time: now, Source: "Sobatin's warder", Target: "an orc raider", Amount: 4})
	meter.Add(Event{Time: now.Add(time.Second), Source: "Sobatin", Target: "an orc raider", Amount: 11})

	players := meter.Players()
	if len(players) != 1 || players[0].Name != "Sobatin" || players[0].Damage != 15 {
		t.Fatalf("unexpected merged stats: %#v", players)
	}
}

func TestFightTrackerStartsNewFightForDifferentMobAfterDeathWithinGracePeriod(t *testing.T) {
	tracker := NewFightTracker()
	now := time.Date(2026, 7, 5, 17, 21, 50, 0, time.UTC)

	tracker.AddDamage(Event{Time: now, Source: "You", Target: "an elemental crusader", Amount: 10})
	tracker.AddDeath(Death{Time: now.Add(time.Second), Victim: "an elemental crusader", Killer: "You"})
	tracker.AddDamage(Event{Time: now.Add(3 * time.Second), Source: "You", Target: "a rock golem", Amount: 20})

	fight, current := tracker.DisplayFight()
	if fight == nil || !current {
		t.Fatalf("expected current fight, got fight=%#v current=%v", fight, current)
	}
	if fight.Meter.Events() != 1 {
		t.Fatalf("expected new fight with one event, got %d", fight.Meter.Events())
	}
	if fight.Meter.Players()[0].LastTarget != "a rock golem" {
		t.Fatalf("expected new fight target, got %#v", fight.Meter.Players())
	}
}

func TestFightTrackerKeepsLateDamageFromSlainMobCompleted(t *testing.T) {
	tracker := NewFightTracker()
	now := time.Date(2026, 7, 5, 17, 22, 20, 0, time.UTC)

	tracker.AddDamage(Event{Time: now, Source: "You", Target: "a rock golem", Amount: 10})
	tracker.AddDeath(Death{Time: now.Add(time.Second), Victim: "a rock golem", Killer: "You"})
	tracker.AddDamage(Event{Time: now.Add(2 * time.Second), Source: "A rock golem", Target: "YOU", Amount: 20})

	fight, current := tracker.DisplayFight()
	if fight == nil || current {
		t.Fatalf("expected completed fight, got fight=%#v current=%v", fight, current)
	}
	if fight.Meter.Events() != 2 {
		t.Fatalf("expected late same-mob damage in completed fight, got %d events", fight.Meter.Events())
	}
}

func TestFightTrackerEndsImmediatelyWhenYouDie(t *testing.T) {
	tracker := NewFightTracker()
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	tracker.AddDamage(Event{Time: now, Source: "You", Target: "a zombie", Amount: 10})
	tracker.AddDeath(Death{Time: now.Add(time.Second), Victim: "You", Killer: "a zombie"})

	fight, current := tracker.DisplayFight()
	if fight == nil || current {
		t.Fatalf("expected completed fight after player death, got fight=%#v current=%v", fight, current)
	}
	if fight.Death.Victim != "You" {
		t.Fatalf("unexpected death: %#v", fight.Death)
	}
}

func TestFightTrackerEndsAfterIdleTimeout(t *testing.T) {
	tracker := NewFightTracker()
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	tracker.AddDamage(Event{Time: now, Source: "You", Target: "a zombie", Amount: 10})
	if tracker.EndIdle(time.Now().Add(DefaultIdleTimeout-time.Second), DefaultIdleTimeout) {
		t.Fatal("did not expect idle end before wall activity is old enough")
	}

	tracker.lastWallSeen = time.Now().Add(-DefaultIdleTimeout - time.Second)
	if !tracker.EndIdle(time.Now(), DefaultIdleTimeout) {
		t.Fatal("expected idle end")
	}

	fight, current := tracker.DisplayFight()
	if fight == nil || current {
		t.Fatalf("expected completed idle fight, got fight=%#v current=%v", fight, current)
	}
	if fight.EndReason != "idle timeout" {
		t.Fatalf("unexpected end reason: %#v", fight)
	}
}

func TestFightTrackerEndsAfterLogTimeIdleTimeout(t *testing.T) {
	tracker := NewFightTracker()
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	tracker.AddDamage(Event{Time: now, Source: "You", Target: "a zombie", Amount: 10})
	if tracker.EndIdleAtLogTime(now.Add(DefaultIdleTimeout-time.Second), DefaultIdleTimeout) {
		t.Fatal("did not expect idle end before log timeout")
	}
	if !tracker.EndIdleAtLogTime(now.Add(DefaultIdleTimeout+time.Second), DefaultIdleTimeout) {
		t.Fatal("expected log-time idle end")
	}

	fight, current := tracker.DisplayFight()
	if fight == nil || current {
		t.Fatalf("expected completed idle fight, got fight=%#v current=%v", fight, current)
	}
	if fight.EndReason != "idle timeout" {
		t.Fatalf("unexpected end reason: %#v", fight)
	}
}

func TestFightTrackerStartsNewFightAfterGracePeriod(t *testing.T) {
	tracker := NewFightTracker()
	now := time.Date(2026, 7, 5, 17, 21, 50, 0, time.UTC)

	tracker.AddDamage(Event{Time: now, Source: "You", Target: "an elemental crusader", Amount: 10})
	tracker.AddDeath(Death{Time: now.Add(time.Second), Victim: "an elemental crusader", Killer: "You"})
	tracker.AddDamage(Event{Time: now.Add(30 * time.Second), Source: "You", Target: "a rock golem", Amount: 20})

	fight, current := tracker.DisplayFight()
	if fight == nil || !current {
		t.Fatalf("expected new current fight, got fight=%#v current=%v", fight, current)
	}
	if fight.Meter.Events() != 1 {
		t.Fatalf("expected new fight with one event, got %d", fight.Meter.Events())
	}
}

func TestFightTrackerKeepsLastThreeCompletedFights(t *testing.T) {
	tracker := NewFightTrackerWithHistory(3)
	now := time.Date(2026, 7, 5, 17, 0, 0, 0, time.UTC)

	for i := range 4 {
		start := now.Add(time.Duration(i) * 2 * deathGracePeriod)
		tracker.AddDamage(Event{
			Time:   start,
			Source: "You",
			Target: "mob",
			Amount: 10 + i,
		})
		tracker.AddDeath(Death{
			Time:   start.Add(time.Second),
			Victim: "mob",
			Killer: "You",
		})
	}
	tracker.AddDamage(Event{
		Time:   now.Add(10 * time.Minute),
		Source: "You",
		Target: "current mob",
		Amount: 99,
	})

	sections := tracker.DisplaySections()
	if len(sections) != 4 {
		t.Fatalf("expected current plus 3 history sections, got %d", len(sections))
	}
	if !sections[0].Current {
		t.Fatalf("expected first section to be current: %#v", sections[0])
	}
}

func TestFightTrackerCanKeepAllCompletedFights(t *testing.T) {
	tracker := NewFightTrackerWithHistory(0)
	now := time.Date(2026, 7, 5, 17, 0, 0, 0, time.UTC)

	for i := range 5 {
		start := now.Add(time.Duration(i) * 2 * deathGracePeriod)
		tracker.AddDamage(Event{
			Time:   start,
			Source: "You",
			Target: "mob",
			Amount: 10 + i,
		})
		tracker.AddDeath(Death{
			Time:   start.Add(time.Second),
			Victim: "mob",
			Killer: "You",
		})
	}
	tracker.AddDamage(Event{
		Time:   now.Add(10 * time.Minute),
		Source: "You",
		Target: "current mob",
		Amount: 99,
	})

	sections := tracker.DisplaySections()
	if len(sections) != 6 {
		t.Fatalf("expected current plus all 5 history sections, got %d", len(sections))
	}
}
