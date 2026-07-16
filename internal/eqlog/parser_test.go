package eqlog

import (
	"testing"
	"time"
)

func TestParseRecordClassifiesTimestampedLines(t *testing.T) {
	tests := []struct {
		name string
		line string
		kind RecordKind
	}{
		{"unknown", "[Thu Jul 02 05:19:00 2026] You say, 'hello'", RecordUnknown},
		{"cast", "[Thu Jul 02 05:19:01 2026] Zonektik begins to cast Furor.", RecordCast},
		{"damage", "[Thu Jul 02 05:19:02 2026] You hit a lizardman scout for 4 points of magic damage by Shallow Breath.", RecordDamage},
		{"experience", "[Thu Jul 02 05:19:03 2026] You gain experience! (1.239%)", RecordExperience},
		{"level up", "[Thu Jul 02 05:19:04 2026] You have gained a level! Welcome to level 43!", RecordLevelUp},
		{"aggro clear", "[Thu Jul 02 05:19:05 2026] Your enemies have forgotten you!", RecordAggroClear},
		{"death", "[Thu Jul 02 05:19:06 2026] You have slain a lizardman scout!", RecordDeath},
		{"zone change", "[Thu Jul 09 08:55:08 2026] You have entered The Plane of Sky.", RecordZoneChange},
		{"loot", "[Thu Jul 16 10:47:28 2026] --You have looted a Wind Rune Caza from Protector of Sky's corpse.--", RecordLoot},
		{"item removal", "[Wed Jul 08 16:04:02 2026] You successfully destroyed 1 Mithril Earring.", RecordItemRemoval},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, ok := ParseRecord(test.line)
			if !ok {
				t.Fatal("expected timestamped record")
			}
			if record.Kind != test.kind {
				t.Fatalf("kind = %v, want %v", record.Kind, test.kind)
			}
			if record.Time.IsZero() {
				t.Fatal("expected record timestamp")
			}
		})
	}
}

func TestParseZoneChangeLine(t *testing.T) {
	zone, ok := ParseZoneChangeLine("[Thu Jul 09 08:55:08 2026] You have entered The Plane of Sky.")
	if !ok || zone.Name != "The Plane of Sky" {
		t.Fatalf("unexpected zone change: %#v, ok=%v", zone, ok)
	}
}

func TestParseLootLineOutcomes(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		item     string
		corpse   string
		quantity int
		outcome  LootOutcome
		created  string
	}{
		{
			"retained rune",
			"[Thu Jul 16 10:47:28 2026] --You have looted a Wind Rune Caza from Protector of Sky's corpse.--",
			"Wind Rune Caza", "Protector of Sky", 1, LootRetained, "",
		},
		{
			"retained stack",
			"[Thu Jul 02 05:34:53 2026] --You have looted 2 Bone Chips from a skeleton's corpse.--",
			"Bone Chips", "a skeleton", 2, LootRetained, "",
		},
		{
			"sold for free",
			"[Thu Jul 16 10:47:17 2026] You looted a Belt of Concordance from Protector of Sky's corpse and sold it for free.",
			"Belt of Concordance", "Protector of Sky", 1, LootSold, "",
		},
		{
			"stored",
			"[Thu Jul 02 05:22:50 2026] You looted a Low Quality Wolf Skin from a shadow wolf's corpse and stored it in your tradeskill depot",
			"Low Quality Wolf Skin", "a shadow wolf", 1, LootStored, "",
		},
		{
			"converted",
			"[Sat Jul 04 10:18:17 2026] You looted a Black Tome with Silver Runes +2 from Najena's corpse to create a Black Tome with Silver Runes +3",
			"Black Tome with Silver Runes +2", "Najena", 1, LootConverted, "a Black Tome with Silver Runes +3",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loot, ok := ParseLootLine(test.line)
			if !ok {
				t.Fatal("expected loot event")
			}
			if loot.Item != test.item || loot.Corpse != test.corpse || loot.Quantity != test.quantity || loot.Outcome != test.outcome || loot.Created != test.created {
				t.Fatalf("loot = %#v", loot)
			}
		})
	}
}

func TestParseItemRemovalLine(t *testing.T) {
	removal, ok := ParseItemRemovalLine("[Wed Jul 08 16:04:02 2026] You successfully destroyed 1 Mithril Earring.")
	if !ok || removal.Item != "Mithril Earring" || removal.Quantity != 1 {
		t.Fatalf("unexpected removal: %#v, ok=%v", removal, ok)
	}
}

func TestParseRecordRejectsInvalidEnvelope(t *testing.T) {
	if record, ok := ParseRecord("not an EverQuest log line"); ok {
		t.Fatalf("unexpected record: %+v", record)
	}
}

func TestParseRecordAfterRejectsOldLineBeforeClassification(t *testing.T) {
	cutoff := time.Date(2026, time.July, 16, 10, 0, 0, 0, time.UTC)
	line := "[Thu Jul 09 09:20:17 2026] --You have looted a Wind Rune Fana from Protector of Sky's corpse.--"
	if record, ok := ParseRecordAfter(line, cutoff); ok {
		t.Fatalf("unexpected old record: %#v", record)
	}

	record, ok := ParseRecordAfter("[Thu Jul 16 10:47:28 2026] --You have looted a Wind Rune Caza from Protector of Sky's corpse.--", cutoff)
	if !ok || record.Kind != RecordLoot || record.Loot.Item != "Wind Rune Caza" {
		t.Fatalf("unexpected accepted record: %#v, ok=%v", record, ok)
	}
}

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
	if event.Source != "Lobantik" || event.Target != "a lizardman scout" || event.Amount != 14 || event.Attack != "pierces" {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestParseLineYouSpellDamage(t *testing.T) {
	event, ok := ParseLine("[Thu Jul 02 05:19:03 2026] You hit a lizardman scout for 4 points of magic damage by Shallow Breath.")
	if !ok {
		t.Fatal("expected damage event")
	}
	if event.Source != "You" || event.Amount != 4 || event.Ability != "Shallow Breath" {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestParseCastLine(t *testing.T) {
	tests := []struct {
		line    string
		source  string
		ability string
	}{
		{"[Wed Jul 15 18:53:34 2026] Zonektik begins casting Furor.", "Zonektik", "Furor"},
		{"[Thu Jul 02 05:18:58 2026] You begin casting Shallow Breath.", "You", "Shallow Breath"},
		{"[Thu Jul 02 05:18:58 2026] A wizard begins to cast Ice Comet.", "a wizard", "Ice Comet"},
	}
	for _, test := range tests {
		cast, ok := ParseCastLine(test.line)
		if !ok || cast.Source != test.source || cast.Ability != test.ability {
			t.Fatalf("unexpected cast for %q: %#v, ok=%v", test.line, cast, ok)
		}
	}
}

func TestParseLineYouSingularMeleeVerbs(t *testing.T) {
	for _, line := range []string{
		"[Sun Jul 05 17:21:44 2026] You strike an elemental crusader for 22 points of damage.",
		"[Sun Jul 05 17:21:45 2026] You crush an elemental crusader for 99 points of damage. (Critical)",
		"[Sun Jul 05 17:21:48 2026] You smite an elemental crusader for 38 points of damage.",
		"[Fri Jul 03 11:26:29 2026] You frenzy on a necro neophyte for 32 points of damage.",
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

func TestParseLineRiposteCritical(t *testing.T) {
	event, ok := ParseLine("[Sun Jul 05 10:41:01 2026] You slash a guk ghoul knight for 38 points of damage. (Riposte Critical)")
	if !ok {
		t.Fatal("expected damage event")
	}
	if !event.Critical {
		t.Fatalf("expected riposte critical event: %#v", event)
	}
	if !event.Incidental {
		t.Fatalf("riposte must not initialize engagement: %#v", event)
	}
}

func TestParseLineIncidentalAOEMelee(t *testing.T) {
	for _, line := range []string{
		"[Thu Jul 02 09:19:37 2026] You cleave a plague rat for 8 points of damage.",
		"[Thu Jul 02 09:19:37 2026] You kick a plague rat for 8 points of damage.",
		"[Tue Jul 14 12:53:01 2026] You strike a fire giant warrior for 30 points of damage.",
		"[Tue Jul 14 12:53:48 2026] You punch a fire giant warrior for 39 points of damage.",
	} {
		event, ok := ParseLine(line)
		if !ok || !event.Incidental {
			t.Fatalf("AoE-style melee must not initialize engagement: %#v, ok=%v", event, ok)
		}
	}
}

func TestParseLineThornsDamage(t *testing.T) {
	event, ok := ParseLine("[Sun Jul 05 17:21:42 2026] A rock golem is pierced by YOUR thorns for 20 points of non-melee damage.")
	if !ok {
		t.Fatal("expected thorns damage event")
	}
	if event.Source != "You" || event.Target != "a rock golem" || event.Amount != 20 || event.Ability != "thorns" || !event.Passive {
		t.Fatalf("unexpected event: %#v", event)
	}
	if !event.Incidental {
		t.Fatalf("damage shield must not initialize engagement: %#v", event)
	}
}

func TestParseLineOtherCombatantDamageShield(t *testing.T) {
	tests := []struct {
		line    string
		source  string
		target  string
		ability string
		amount  int
	}{
		{
			line:    "[Sun Jul 05 19:15:54 2026] A fire giant warrior is pierced by Clown's thorns for 29 points of non-melee damage.",
			source:  "Clown",
			target:  "a fire giant warrior",
			ability: "thorns",
			amount:  29,
		},
		{
			line:    "[Mon Jul 06 07:41:35 2026] YOU are burned by a forsaken revenant's flames for 19 points of non-melee damage!",
			source:  "a forsaken revenant",
			target:  "YOU",
			ability: "flames",
			amount:  19,
		},
		{
			line:    "[Wed Jul 08 17:21:17 2026] YOU are pierced by Innoruuk's Chosen's thorns for 13 points of non-melee damage!",
			source:  "Innoruuk's Chosen",
			target:  "YOU",
			ability: "thorns",
			amount:  13,
		},
	}

	for _, test := range tests {
		event, ok := ParseLine(test.line)
		if !ok {
			t.Fatalf("expected damage event for %q", test.line)
		}
		if event.Source != test.source || event.Target != test.target || event.Ability != test.ability || event.Amount != test.amount {
			t.Fatalf("unexpected event for %q: %#v", test.line, event)
		}
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

func TestParseLineIncomingDotDamage(t *testing.T) {
	event, ok := ParseLine("[Fri Jul 10 14:14:43 2026] You have taken 10 damage from Poison by a deadly black widow.")
	if !ok {
		t.Fatal("expected damage event")
	}
	if event.Source != "a deadly black widow" || event.Target != "YOU" || event.Amount != 10 || event.Ability != "Poison" {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestParseLineCriticalDotDamage(t *testing.T) {
	event, ok := ParseLine("[Sun Jul 05 19:15:54 2026] A lava duct crawler has taken 36 damage from Denon's Disruptive Discord by Clown. (Critical)")
	if !ok {
		t.Fatal("expected damage event")
	}
	if event.Source != "Clown" || event.Target != "a lava duct crawler" || event.Amount != 36 || event.Ability != "Denon's Disruptive Discord" || !event.Critical {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestParseLineDotWithoutSourceIsIgnored(t *testing.T) {
	if event, ok := ParseLine("[Sun Jul 05 19:15:54 2026] A fire giant warrior has taken 18 damage by Denon's Disruptive Discord."); ok {
		t.Fatalf("expected source-less damage to be ignored, got %#v", event)
	}
}

func TestParseLineAdditionalMeleeVerbs(t *testing.T) {
	lines := []string{
		"[Sun Jul 05 20:49:40 2026] Bigg reaves a fire giant warrior for 24 points of damage.",
		"[Sun Jul 05 19:04:17 2026] A watchful guard smashes YOU for 28 points of damage.",
	}
	for _, line := range lines {
		if _, ok := ParseLine(line); !ok {
			t.Fatalf("expected damage event for %q", line)
		}
	}
}

func TestParseLineYourDotDamage(t *testing.T) {
	event, ok := ParseLine("[Mon Jul 13 09:08:08 2026] A zol ghoul knight has taken 49 damage from your Tuyen's Chant of Flame.")
	if !ok {
		t.Fatal("expected damage event")
	}
	if event.Source != "You" || event.Target != "a zol ghoul knight" || event.Amount != 49 || event.Ability != "Tuyen's Chant of Flame" || !event.Passive || !event.DamageOverTime {
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

func TestParseExperienceLine(t *testing.T) {
	gain, ok := ParseExperienceLine("[Mon Jul 13 16:46:49 2026] You gain experience! (1.239%)")
	if !ok {
		t.Fatal("expected experience gain")
	}
	if gain.Percent != 1.239 {
		t.Fatalf("unexpected experience percentage: %#v", gain)
	}
	want := time.Date(2026, 7, 13, 16, 46, 49, 0, time.UTC)
	if !gain.Time.Equal(want) {
		t.Fatalf("expected timestamp %s, got %s", want, gain.Time)
	}
}

func TestParseLevelUpLine(t *testing.T) {
	levelUp, ok := ParseLevelUpLine("[Mon Jul 13 15:34:31 2026] You have gained a level! Welcome to level 43!")
	if !ok {
		t.Fatal("expected level-up event")
	}
	if levelUp.Level != 43 {
		t.Fatalf("unexpected level-up: %#v", levelUp)
	}
	want := time.Date(2026, 7, 13, 15, 34, 31, 0, time.UTC)
	if !levelUp.Time.Equal(want) {
		t.Fatalf("expected timestamp %s, got %s", want, levelUp.Time)
	}
}

func TestParseAggroClearLine(t *testing.T) {
	want := time.Date(2026, 7, 13, 14, 56, 50, 0, time.UTC)
	got, ok := ParseAggroClearLine("[Mon Jul 13 14:56:50 2026] Your enemies have forgotten you!")
	if !ok {
		t.Fatal("expected aggro-clear event")
	}
	if !got.Equal(want) {
		t.Fatalf("expected timestamp %s, got %s", want, got)
	}
}
