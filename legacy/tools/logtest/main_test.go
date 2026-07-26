package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunAppendsTextToExplicitLogfile(t *testing.T) {
	logFile := emptyLogfile(t)
	now := time.Date(2026, time.July, 24, 21, 5, 9, 0, time.Local)

	if err := run([]string{"-log", logFile, "-t", "Some random Text"}, now); err != nil {
		t.Fatal(err)
	}
	assertLogContents(t, logFile, "[Fri Jul 24 21:05:09 2026] Some random Text\n")
}

func TestRunAppendsSpellFadeMessage(t *testing.T) {
	logFile := emptyLogfile(t)
	now := time.Date(2026, time.July, 24, 21, 5, 9, 0, time.Local)

	if err := run([]string{"-s", "Spirit of Wolf", "-log", logFile}, now); err != nil {
		t.Fatal(err)
	}
	assertLogContents(t, logFile, "[Fri Jul 24 21:05:09 2026] The spirit of wolf leaves you.\n")
}

func TestRunRejectsUnknownSpell(t *testing.T) {
	logFile := emptyLogfile(t)
	err := run([]string{"-log", logFile, "-s", "This spell does not exist"}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "not in the embedded catalogue") {
		t.Fatalf("error = %v", err)
	}
	assertLogContents(t, logFile, "")
}

func TestRunRequiresOneTriggerAndLogfile(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"-t", "text"},
		{"-log", "/tmp/log", "-t", "text", "-s", "spell"},
	} {
		if err := run(args, time.Now()); err == nil || err.Error() != logtestUsage {
			t.Errorf("run(%q) error = %v, want usage", args, err)
		}
	}
}

func emptyLogfile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "eqlog_Test_server.txt")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertLogContents(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != want {
		t.Fatalf("logfile = %q, want %q", got, want)
	}
}
