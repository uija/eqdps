package eqlog

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/uija/eqdps/internal/data"
)

func TestParseRowClassifiesSharedEventPatterns(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		eventType data.LogRowEventType
	}{
		{"cast", "Zonektik begins casting Furor.", data.LogRowEventTypeCast},
		{"damage", "You pierce an elemental visier for 44 points of damage.", data.LogRowEventTypeDamage},
		{"your damage shield", "A rock golem is pierced by YOUR thorns for 20 points of non-melee damage.", data.LogRowEventTypeYourDamageShield},
		{"damage shield", "A fire giant warrior is pierced by Clown's thorns for 29 points of non-melee damage.", data.LogRowEventTypeDamageShield},
		{"your damage over time", "A zol ghoul knight has taken 49 damage from your Tuyen's Chant of Flame.", data.LogRowEventTypeYourDamageOverTime},
		{"damage over time", "An orc raider has taken 2 damage from Flame Lick by Sobatin.", data.LogRowEventTypeDamageOverTime},
		{"experience", "You gain experience! (1.239%)", data.LogRowEventTypeExperience},
		{"party experience", "You gain party experience! (0.283%)", data.LogRowEventTypeExperience},
		{"kill experience", "You gain party experience!", data.LogRowEventTypeKillExperienceReward},
		{"corpse coin", "You receive 2 gold from the corpse.", data.LogRowEventTypeCorpseCoinReward},
		{"level up", "You have gained a level! Welcome to level 43!", data.LogRowEventTypeLevelUp},
		{"aggro clear", "Your enemies have forgotten you!", data.LogRowEventTypeAggroClear},
		{"you slain", "You have slain a fire giant warrior!", data.LogRowEventTypeYouSlain},
		{"player slain", "You have been slain by a fire giant warrior!", data.LogRowEventTypeSlainBy},
		{"slain by", "A fire giant warrior has been slain by Wyrmberg!", data.LogRowEventTypeSlainBy},
		{"zone", "You have entered The Plane of Sky.", data.LogRowEventTypeZoneChange},
		{"loot", "--You have looted a Wind Rune Caza from a thunder spirit's corpse.--", data.LogRowEventTypeLoot},
		{"loot result", "You looted a Wind Rune Caza from a thunder spirit's corpse and stored it in your bank", data.LogRowEventTypeLootResult},
		{"destroyed", "You successfully destroyed 1 Wind Rune Caza.", data.LogRowEventTypeItemDestroyed},
		{"trade offer", "You offered 1 Wind Rune Caza to Cilin Spellsinger.", data.LogRowEventTypeTradeOffer},
		{"trade complete", "You complete the trade with Cilin Spellsinger.", data.LogRowEventTypeTradeComplete},
		{"trade cancel", "Cilin Spellsinger has cancelled the trade.", data.LogRowEventTypeTradeCancel},
		{"who", "[50 PAL/DRU/MNK] Wyrmberg (Dwarf) <Guild> ZONE:", data.LogRowEventTypeWho},
		{"anonymous who", "[ANONYMOUS] Someone", data.LogRowEventTypeAnonymousWho},
		{"inventory export", "Outputfile Complete: Wyrmberg_rivervale-Inventory.txt", data.LogRowEventTypeInventoryExport},
	}

	parser := NewParser(0)
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := "[Sun Jul 26 12:00:00 2026] " + test.message
			event, ok := parser.ParseRow(row, int64(index+1), false)
			if !ok {
				t.Fatal("row was not parsed")
			}
			if event.Type != test.eventType {
				t.Fatalf("type = %v, want %v", event.Type, test.eventType)
			}
			if len(event.Data) == 0 || event.Data[0] != test.message {
				t.Fatalf("regexp data = %#v", event.Data)
			}
		})
	}
}

func TestParseRowPlayerDeathPreservesVictimAndKiller(t *testing.T) {
	const row = "[Sun Jul 26 12:00:00 2026] You have been slain by a fire giant warrior!"

	event, ok := NewParser(0).ParseRow(row, int64(len(row)), true)
	if !ok {
		t.Fatal("player death row was not parsed")
	}
	if event.Type != data.LogRowEventTypeSlainBy {
		t.Fatalf("type = %v, want %v", event.Type, data.LogRowEventTypeSlainBy)
	}
	if got, want := event.Data[1], "You"; got != want {
		t.Fatalf("victim = %q, want %q", got, want)
	}
	if got, want := event.Data[2], "a fire giant warrior"; got != want {
		t.Fatalf("killer = %q, want %q", got, want)
	}
}

func TestParseRowEmitsUnknownRowsOnlyWhenLive(t *testing.T) {
	const row = "[Thu Aug 13 23:55:24 2026] Your Charm spell has worn off of a Teir`Dal rogue."
	parser := NewParser(0)

	if event, ok := parser.ParseRow(row, int64(len(row)), false); ok || event != nil {
		t.Fatalf("unknown replay row was emitted: %#v", event)
	}

	event, ok := parser.ParseRow(row, int64(len(row)), true)
	if !ok || event == nil {
		t.Fatal("unknown live row was not emitted")
	}
	if event.Type != data.LogRowEventTypeUnknown {
		t.Fatalf("type = %v, want %v", event.Type, data.LogRowEventTypeUnknown)
	}
	if !event.Live {
		t.Fatal("unknown followed row was not marked live")
	}
	if event.Message != "Your Charm spell has worn off of a Teir`Dal rogue." {
		t.Fatalf("message = %q", event.Message)
	}
}

func TestReplayBuildsMetadataAndOffsets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eqlog_Wyrmberg_rivervale.txt")
	rows := []string{
		"[Sun Jul 26 12:00:00 2026] You gain experience! (1%)",
		"[Sun Jul 26 12:00:01 2026] [50 PAL/DRU/MNK] Wyrmberg (Dwarf) <Guild> ZONE:",
		"[Sun Jul 26 12:00:02 2026] You pierce an elemental visier for 44 points of damage.",
		"[Sun Jul 26 12:00:03 2026] You have gained a level! Welcome to level 51!",
	}
	content := ""
	for _, row := range rows {
		content += row + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	parser := NewParser(0)
	if err := parser.Open(path); err != nil {
		t.Fatal(err)
	}
	var events []*data.LogRowEvent
	var progress []ReplayProgress
	if err := parser.Replay(Loopback{}, func(event *data.LogRowEvent) {
		events = append(events, event)
	}, func(update ReplayProgress) {
		progress = append(progress, update)
	}); err != nil {
		t.Fatal(err)
	}

	if len(events) != len(rows) {
		t.Fatalf("events = %d, want %d", len(events), len(rows))
	}
	if events[0].Type != data.LogRowEventTypeExperience {
		t.Fatalf("first type = %v", events[0].Type)
	}
	if events[0].Metadata.CharacterName != "Wyrmberg" || events[0].Metadata.ServerName != "rivervale" {
		t.Fatalf("identity = %#v", events[0].Metadata)
	}
	if events[0].Metadata.Level != 0 {
		t.Fatalf("level before /who = %d", events[0].Metadata.Level)
	}
	who := events[1].Metadata
	if who.Level != 50 || who.Race != "Dwarf" || !slices.Equal(who.Classes, []string{"PAL", "DRU", "MNK"}) {
		t.Fatalf("/who metadata = %#v", who)
	}
	if !who.WhoObservedAt.Equal(events[1].Timestamp) {
		t.Fatalf("/who observation = %v, want %v", who.WhoObservedAt, events[1].Timestamp)
	}
	if events[3].Metadata.Level != 51 {
		t.Fatalf("level after level-up = %d", events[3].Metadata.Level)
	}
	if !events[3].Metadata.WhoObservedAt.Equal(who.WhoObservedAt) {
		t.Fatal("level-up changed the last /who observation time")
	}
	if events[len(events)-1].Offset != int64(len(content)) {
		t.Fatalf("final offset = %d, want %d", events[len(events)-1].Offset, len(content))
	}
	for _, event := range events {
		if event.Live {
			t.Fatal("replayed event marked live")
		}
	}
	if len(progress) != 2 {
		t.Fatalf("progress updates = %#v", progress)
	}
	if progress[0] != (ReplayProgress{Total: int64(len(content))}) {
		t.Fatalf("initial progress = %#v", progress[0])
	}
	if progress[1] != (ReplayProgress{Bytes: int64(len(content)), Total: int64(len(content)), Lines: len(rows)}) {
		t.Fatalf("final progress = %#v", progress[1])
	}
}

func TestReplayUsesLookbackFromLatestLogTimestamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eqlog_Wyrmberg_rivervale.txt")
	rows := []string{
		"[Sun Jul 26 12:00:00 2026] You gain experience! (1%)",
		"[Sun Jul 26 12:05:00 2026] You gain experience! (2%)",
		"[Sun Jul 26 12:10:00 2026] You gain experience! (3%)",
	}
	content := ""
	for _, row := range rows {
		content += row + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	parser := NewParser(0)
	if err := parser.Open(path); err != nil {
		t.Fatal(err)
	}
	var events []*data.LogRowEvent
	if err := parser.Replay(Loopback{TimeOffset: 6 * time.Minute}, func(event *data.LogRowEvent) {
		events = append(events, event)
	}, nil); err != nil {
		t.Fatal(err)
	}

	if len(events) != 2 || events[0].Message != "You gain experience! (2%)" || events[1].Message != "You gain experience! (3%)" {
		t.Fatalf("events = %#v", events)
	}
	if events[1].Offset != int64(len(content)) {
		t.Fatalf("final offset = %d, want %d", events[1].Offset, len(content))
	}
}

func TestLatestTimestampUsesLastCompleteRowFromFileTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eqlog_Wyrmberg_rivervale.txt")
	var content strings.Builder
	content.WriteString("[Sun Jul 26 12:00:00 2099] Timestamp outside the tail.\n")
	for content.Len() < int(latestTimestampTailBytes)+2048 {
		content.WriteString("[Sun Jul 26 11:00:00 2026] Filler row.\n")
	}
	content.WriteString("[Sun Jul 26 12:10:00 2026] Last complete row.\n")
	content.WriteString("[Sun Jul 26 12:11:00 2026] Incomplete row")
	if err := os.WriteFile(path, []byte(content.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	latest, stopped, err := latestTimestamp(file, info.Size(), make(chan struct{}))
	if err != nil {
		t.Fatal(err)
	}
	if stopped {
		t.Fatal("tail lookup unexpectedly stopped")
	}
	want := time.Date(2026, time.July, 26, 12, 10, 0, 0, time.UTC)
	if !latest.Equal(want) {
		t.Fatalf("latest timestamp = %v, want %v", latest, want)
	}
}

func TestFollowStartsAtOpenOffsetAndStopsOnClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eqlog_Wyrmberg_rivervale.txt")
	initial := "[Sun Jul 26 12:00:00 2026] Existing row.\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	parser := NewParser(0)
	if err := parser.Open(path); err != nil {
		t.Fatal(err)
	}
	events := make(chan *data.LogRowEvent, 1)
	followDone := make(chan error, 1)
	go func() {
		followDone <- parser.Follow(func(event *data.LogRowEvent) {
			events <- event
		})
	}()

	appended := "[Sun Jul 26 12:00:01 2026] You gain experience! (1.000%)\n"
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(appended); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-events:
		if !event.Live {
			t.Fatal("followed event was not marked live")
		}
		if event.Type != data.LogRowEventTypeExperience {
			t.Fatalf("type = %v", event.Type)
		}
		if event.Offset != int64(len(initial)+len(appended)) {
			t.Fatalf("offset = %d, want %d", event.Offset, len(initial)+len(appended))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for followed row")
	}

	parser.Close()
	select {
	case err := <-followDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("follow did not stop")
	}
}

func TestParserRequiresOpenLog(t *testing.T) {
	parser := NewParser(0)
	if err := parser.Replay(Loopback{}, nil, nil); !errors.Is(err, ErrLogNotOpen) {
		t.Fatalf("Replay error = %v", err)
	}
	if err := parser.Follow(nil); !errors.Is(err, ErrLogNotOpen) {
		t.Fatalf("Follow error = %v", err)
	}
}

func TestCloseClearsOpenedLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eqlog_Wyrmberg_rivervale.txt")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	parser := NewParser(0)
	if err := parser.Open(path); err != nil {
		t.Fatal(err)
	}
	parser.Close()
	if err := parser.Replay(Loopback{}, nil, nil); !errors.Is(err, ErrLogNotOpen) {
		t.Fatalf("Replay error after Close = %v", err)
	}
}
