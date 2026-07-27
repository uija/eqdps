package launchlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uija/eqdps/internal/dropcollector"
	"github.com/uija/eqdps/internal/eqldbqueue"
)

func TestInspectAndRememberIgnore(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv(cutoffEnvironment, defaultCutoffText)
	logPath := filepath.Join(t.TempDir(), "eqlog_Test_Server.txt")
	writeTestLog(t, logPath,
		"[Sun Jul 26 23:59:59 2026] beta\n",
		"[Mon Jul 27 00:00:00 2026] launch\n",
	)

	check, err := Inspect(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !check.NeedsAction {
		t.Fatal("beta logfile did not require a decision")
	}
	if err := RememberIgnored(logPath); err != nil {
		t.Fatal(err)
	}
	check, err = Inspect(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if check.NeedsAction {
		t.Fatal("remembered ignore decision was not applied")
	}
}

func TestFixArchivesBetaAndResetsDerivedState(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv(cutoffEnvironment, defaultCutoffText)
	logPath := filepath.Join(t.TempDir(), "eqlog_Test_Server.txt")
	beta := "[Sun Jul 26 23:59:59 2026] beta\n"
	launch := "[Mon Jul 27 00:00:00 2026] launch\n[Mon Jul 27 00:00:01 2026] more\n"
	writeTestLog(t, logPath, beta, launch)

	skyPath := filepath.Join(filepath.Dir(logPath), "Test_Server_PoS.json")
	if err := os.WriteFile(skyPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	collector, err := dropcollector.Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := collector.Close(); err != nil {
		t.Fatal(err)
	}
	collectorStates, err := filepath.Glob(filepath.Join(config, "eqdps", "drop-collection", "*-state.json"))
	if err != nil || len(collectorStates) != 1 {
		t.Fatalf("drop collector states = %v, %v", collectorStates, err)
	}
	collectorState := collectorStates[0]
	queue, err := eqldbqueue.Default()
	if err != nil {
		t.Fatal(err)
	}
	before := time.Date(2026, time.July, 26, 23, 0, 0, 0, time.UTC)
	after := time.Date(2026, time.July, 27, 1, 0, 0, 0, time.UTC)
	if err := queue.Append(eqldbqueue.Kills, "beta", struct {
		Timestamp time.Time `json:"timestamp"`
	}{before}); err != nil {
		t.Fatal(err)
	}
	if err := queue.Append(eqldbqueue.Kills, "launch", struct {
		Timestamp time.Time `json:"timestamp"`
	}{after}); err != nil {
		t.Fatal(err)
	}

	archive, err := Fix(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(archive); err != nil || string(data) != beta+launch {
		t.Fatalf("archive = %q, %v", data, err)
	}
	if data, err := os.ReadFile(logPath); err != nil || string(data) != launch {
		t.Fatalf("launch logfile = %q, %v", data, err)
	}
	if _, err := os.Stat(skyPath); !os.IsNotExist(err) {
		t.Fatalf("active Plane of Sky state still exists: %v", err)
	}
	if _, err := os.Stat(strings.TrimSuffix(skyPath, ".json") + ".beta.json"); err != nil {
		t.Fatalf("archived Plane of Sky state missing: %v", err)
	}
	if _, err := os.Stat(collectorState); !os.IsNotExist(err) {
		t.Fatalf("drop collector state still exists: %v", err)
	}
	entries, err := queue.Batch(eqldbqueue.Kills, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != "launch" {
		data, _ := json.Marshal(entries)
		t.Fatalf("retained queue entries = %s", data)
	}
}

func writeTestLog(t *testing.T, path string, parts ...string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(parts, "")), 0o600); err != nil {
		t.Fatal(err)
	}
}
