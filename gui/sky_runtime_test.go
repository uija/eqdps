package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uija/eqdps/internal/skyquest"
)

func TestMissingSkyStateOpensFirstUseConsent(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "eqlog_Test_Server.txt")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	database := skyquest.Database{SchemaVersion: 1, Classes: []skyquest.Class{{
		Name: "Bard", Quests: []skyquest.Quest{{Name: "Bard Test", Requirements: []skyquest.Requirement{{Name: "Rune", Quantity: 1}}}},
	}}}
	shell := shell{skyDatabase: database, skyInventory: make(map[string]int)}
	shell.loadSkyState(logPath)
	if !shell.skySetupOpen {
		t.Fatal("expected first-use Plane of Sky consent")
	}
	if exists, err := skyquest.StateExists(logPath); err != nil || exists {
		t.Fatalf("consent prompt must not create state: exists=%v err=%v", exists, err)
	}
}

func TestDeniedSkyTrackingDoesNotPromptAgainThisRun(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "eqlog_Test_Server.txt")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	shell := shell{skyDatabase: skyquest.Database{SchemaVersion: 1}, skyDenied: true}
	shell.loadSkyState(logPath)
	if shell.skySetupOpen {
		t.Fatal("did not expect another consent prompt after denial in the same run")
	}
}

func TestSkyAsyncUpdateIgnoresPreviousLogfile(t *testing.T) {
	shell := shell{currentLog: "/current"}
	shell.applySkyAsyncUpdate(skyAsyncUpdate{path: "/old", message: "stale"})
	if shell.skyMessage == "stale" {
		t.Fatal("stale logfile update was applied")
	}
}

func TestFormatSkyBytes(t *testing.T) {
	if got := formatSkyBytes(2 * 1024 * 1024); got != "2.0 MiB" {
		t.Fatalf("unexpected byte text: %q", got)
	}
}

func TestNewReadyQuestCreatesTemporaryNotice(t *testing.T) {
	before := map[string]struct{}{}
	shell := shell{skyProgress: []skyquest.QuestProgress{{
		Class: "Bard", Quest: skyquest.Quest{Name: "Test of Voice"}, Ready: true,
	}}}
	shell.notifyNewReadyQuests(before)
	if shell.skyNoticeText != "PoS: 1 ready · New turn-in available" {
		t.Fatalf("unexpected notice: %q", shell.skyNoticeText)
	}
	if !shell.skyNoticeUntil.After(time.Now()) {
		t.Fatal("expected temporary notice expiration in the future")
	}
}

func TestExistingReadyQuestDoesNotCreateAnotherNotice(t *testing.T) {
	progress := skyquest.QuestProgress{Class: "Bard", Quest: skyquest.Quest{Name: "Test of Voice"}, Ready: true}
	shell := shell{skyProgress: []skyquest.QuestProgress{progress}}
	shell.notifyNewReadyQuests(shell.skyReadyQuestKeys())
	if shell.skyNoticeText != "" {
		t.Fatalf("unexpected repeat notice: %q", shell.skyNoticeText)
	}
}
