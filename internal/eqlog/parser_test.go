package eqlog

import "testing"

func TestParseTime(t *testing.T) {
	timestamp, ok := ParseTime("[Thu Jul 02 05:19:07 2026] Lobantik pierces a lizardman scout for 14 points of damage.")
	if !ok {
		t.Fatal("expected timestamp")
	}
	if timestamp.Year() != 2026 || timestamp.Hour() != 5 || timestamp.Minute() != 19 {
		t.Fatalf("unexpected timestamp: %v", timestamp)
	}
}

func TestParseLinePlayerMelee(t *testing.T) {
	event, ok := ParseLine("[Thu Jul 02 05:19:07 2026] Lobantik pierces a lizardman scout for 14 points of damage.")
	if !ok {
		t.Fatal("expected damage event")
	}
	if event.Source != "Lobantik" || event.Target != "a lizardman scout" || event.Amount != 14 {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestParseLineYouSpellDamage(t *testing.T) {
	event, ok := ParseLine("[Thu Jul 02 05:19:03 2026] You hit a lizardman scout for 4 points of magic damage by Shallow Breath.")
	if !ok {
		t.Fatal("expected damage event")
	}
	if event.Source != "You" || event.Amount != 4 || event.Kind != "magic damage" || event.Ability != "Shallow Breath" {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestParseLineYouSingularMeleeVerbs(t *testing.T) {
	for _, line := range []string{
		"[Sun Jul 05 17:21:44 2026] You strike an elemental crusader for 22 points of damage.",
		"[Sun Jul 05 17:21:45 2026] You crush an elemental crusader for 99 points of damage. (Critical)",
		"[Sun Jul 05 17:21:48 2026] You smite an elemental crusader for 38 points of damage.",
	} {
		if _, ok := ParseLine(line); !ok {
			t.Fatalf("expected damage event for %q", line)
		}
	}
}

func TestParseLineFinishingBlow(t *testing.T) {
	event, ok := ParseLine("[Sun Jul 05 17:21:54 2026] You slash an elemental crusader for 138 points of damage. (Finishing Blow)")
	if !ok {
		t.Fatal("expected damage event")
	}
	if event.Critical {
		t.Fatalf("finishing blow should not be marked critical: %#v", event)
	}
}

func TestParseLineThornsDamage(t *testing.T) {
	event, ok := ParseLine("[Sun Jul 05 17:21:42 2026] A rock golem is pierced by YOUR thorns for 20 points of non-melee damage.")
	if !ok {
		t.Fatal("expected thorns damage event")
	}
	if event.Source != "You" || event.Target != "a rock golem" || event.Amount != 20 || event.Ability != "thorns" {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestParseLineNormalizesLeadingArticleCase(t *testing.T) {
	outgoing, ok := ParseLine("[Sun Jul 05 17:21:44 2026] You slash an ire ghast for 54 points of damage.")
	if !ok {
		t.Fatal("expected outgoing damage event")
	}
	incoming, ok := ParseLine("[Sun Jul 05 17:21:45 2026] An ire ghast hits YOU for 72 points of damage.")
	if !ok {
		t.Fatal("expected incoming damage event")
	}
	if outgoing.Target != incoming.Source {
		t.Fatalf("expected normalized names to match, got %q and %q", outgoing.Target, incoming.Source)
	}
}

func TestParseLineCritical(t *testing.T) {
	event, ok := ParseLine("[Thu Jul 02 05:19:23 2026] Corlan slashes a lizardman warrior for 29 points of damage. (Critical)")
	if !ok {
		t.Fatal("expected damage event")
	}
	if !event.Critical {
		t.Fatalf("expected critical event: %#v", event)
	}
}

func TestParseLineIncludesIncomingNPCDamage(t *testing.T) {
	event, ok := ParseLine("[Thu Jul 02 05:19:07 2026] A lizardman scout hits YOU for 6 points of damage.")
	if !ok {
		t.Fatal("expected incoming NPC damage event")
	}
	if event.Source != "a lizardman scout" || event.Target != "YOU" || event.Amount != 6 {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestParseLineDotDamage(t *testing.T) {
	event, ok := ParseLine("[Thu Jul 02 08:09:13 2026] An orc raider has taken 2 damage from Flame Lick by Sobatin.")
	if !ok {
		t.Fatal("expected damage event")
	}
	if event.Source != "Sobatin" || event.Target != "an orc raider" || event.Amount != 2 || event.Ability != "Flame Lick" {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestParseLinePetDamageAttributesOwner(t *testing.T) {
	event, ok := ParseLine("[Thu Jul 02 08:09:17 2026] Sobatin`s warder crushes an orc raider for 4 points of damage.")
	if !ok {
		t.Fatal("expected damage event")
	}
	if event.Source != "Sobatin`s warder" || event.Amount != 4 {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestParseLinePossessiveMobNameIsNotPetOwner(t *testing.T) {
	event, ok := ParseLine("[Wed Jul 08 17:21:17 2026] Innoruuk`s Chosen hits YOU for 37 points of damage.")
	if !ok {
		t.Fatal("expected damage event")
	}
	if event.Source != "Innoruuk`s Chosen" {
		t.Fatalf("unexpected source: %#v", event)
	}
}

func TestParseDeathLineYouHaveSlain(t *testing.T) {
	death, ok := ParseDeathLine("[Thu Jul 02 05:19:08 2026] You have slain a lizardman scout!")
	if !ok {
		t.Fatal("expected death event")
	}
	if death.Victim != "a lizardman scout" || death.Killer != "You" {
		t.Fatalf("unexpected death: %#v", death)
	}
}

func TestParseDeathLineSlainBy(t *testing.T) {
	death, ok := ParseDeathLine("[Thu Jul 02 05:22:45 2026] A shadow wolf has been slain by Lobantik!")
	if !ok {
		t.Fatal("expected death event")
	}
	if death.Victim != "a shadow wolf" || death.Killer != "Lobantik" {
		t.Fatalf("unexpected death: %#v", death)
	}
}
