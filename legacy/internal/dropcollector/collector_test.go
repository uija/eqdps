package dropcollector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/uija/eqdps/legacy/internal/eqldbqueue"
)

func TestCollectorRecordsRelevantKillAndLootOnlyOnceAcrossReloads(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	logPath := filepath.Join(t.TempDir(), "eqlog_Wyrmberg_rivervale.txt")
	initial := "[Sun Jul 05 20:55:00 2026] You have entered Nagafen's Lair.\n"
	if err := os.WriteFile(logPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	collector, err := Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := collector.SetEnabled(true); err != nil {
		t.Fatal(err)
	}

	appended := "" +
		"[Sun Jul 05 20:56:00 2026] You gain party experience! (0.001%)\n" +
		"[Sun Jul 05 20:56:00 2026] You receive 1 platinum from the corpse.\n" +
		"[Sun Jul 05 20:56:00 2026] Lord Nagafen has been slain by Clown!\n" +
		"[Sun Jul 05 20:57:00 2026] --You have looted a Red Dragon Scale from Lord Nagafen's corpse.--\n"
	appendFile(t, logPath, appended)
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := collector.Sync(info.Size()); err != nil {
		t.Fatal(err)
	}
	if err := collector.Sync(info.Size()); err != nil {
		t.Fatal(err)
	}
	if err := collector.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Sync(info.Size()); err != nil {
		t.Fatal(err)
	}
	observations := append(readObservations(t, reopened.queue, eqldbqueue.Kills), readObservations(t, reopened.queue, eqldbqueue.Drops)...)
	if len(observations) != 2 {
		t.Fatalf("got %d observations, want one kill and one loot: %#v", len(observations), observations)
	}
	if observations[0].Kind != "kill" || observations[0].Mob != "Lord Nagafen" {
		t.Fatalf("unexpected kill observation: %#v", observations[0])
	}
	if observations[1].Kind != "loot" || observations[1].Item != "Red Dragon Scale" {
		t.Fatalf("unexpected loot observation: %#v", observations[1])
	}
}

func TestDelayedLootConfirmsAllMatchingPendingKills(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	logPath := filepath.Join(t.TempDir(), "eqlog_Wyrmberg_rivervale.txt")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	collector, err := Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := collector.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	lines := "" +
		"[Sun Jul 26 12:00:00 2026] You have entered North Ro.\n" +
		"[Sun Jul 26 12:00:01 2026] a zombie has been slain by Moth!\n" +
		"[Sun Jul 26 12:00:02 2026] a zombie has been slain by Gigglemage!\n" +
		"[Sun Jul 26 12:02:00 2026] --You have looted a rusty dagger from a zombie's corpse.--\n"
	appendFile(t, logPath, lines)
	info, _ := os.Stat(logPath)
	if err := collector.Sync(info.Size()); err != nil {
		t.Fatal(err)
	}
	observations := readObservations(t, collector.queue, eqldbqueue.Kills)
	kills := 0
	for _, observation := range observations {
		if observation.Kind == "kill" {
			kills++
		}
	}
	if kills != 2 {
		t.Fatalf("delayed personal loot confirmed %d matching kills, want 2", kills)
	}
}

func TestOptedOutPeriodIsNotCollectedRetroactively(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	logPath := filepath.Join(t.TempDir(), "eqlog_Wyrmberg_rivervale.txt")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	collector, err := Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	appendFile(t, logPath, "[Sun Jul 26 12:00:00 2026] You have slain a decaying skeleton!\n")
	if err := collector.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(logPath)
	if err := collector.Sync(info.Size()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{eqldbqueue.Kills, eqldbqueue.Drops} {
		if _, err := os.Stat(collector.queue.Path(name)); !os.IsNotExist(err) {
			t.Fatalf("pre-opt-in activity was collected in %s, stat error: %v", name, err)
		}
	}
}

func appendFile(t *testing.T, path, value string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(value); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func readObservations(t *testing.T, queue *eqldbqueue.Queue, name string) []Observation {
	t.Helper()
	entries, err := queue.Batch(name, 100)
	if err != nil {
		t.Fatal(err)
	}
	var observations []Observation
	for _, entry := range entries {
		var observation Observation
		if err := json.Unmarshal(entry.Payload, &observation); err != nil {
			t.Fatal(err)
		}
		observations = append(observations, observation)
	}
	return observations
}
