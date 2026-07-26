package eqldbsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uija/eqdps/legacy/internal/dropcollector"
	"github.com/uija/eqdps/legacy/internal/eqldb"
	"github.com/uija/eqdps/legacy/internal/eqldbqueue"
	"github.com/uija/eqdps/legacy/internal/skyquest"
)

type fakeAPI struct {
	sky   []eqldb.PlaneOfSkyEvent
	kills []eqldb.KillObservation
	drops []eqldb.DropObservation
	err   error
}

type signalingAPI struct {
	sky chan struct{}
}

func (f *signalingAPI) SubmitPlaneOfSkyEvents(context.Context, string, string, string, []eqldb.PlaneOfSkyEvent) error {
	select {
	case f.sky <- struct{}{}:
	default:
	}
	return nil
}

func (*signalingAPI) SubmitKillObservations(context.Context, string, []eqldb.KillObservation) error {
	return nil
}

func (*signalingAPI) SubmitDropObservations(context.Context, string, []eqldb.DropObservation) error {
	return nil
}

func (f *fakeAPI) SubmitPlaneOfSkyEvents(_ context.Context, _, _, _ string, events []eqldb.PlaneOfSkyEvent) error {
	if f.err != nil {
		return f.err
	}
	f.sky = append(f.sky, events...)
	return nil
}

func (f *fakeAPI) SubmitKillObservations(_ context.Context, _ string, events []eqldb.KillObservation) error {
	if f.err != nil {
		return f.err
	}
	f.kills = append(f.kills, events...)
	return nil
}

func (f *fakeAPI) SubmitDropObservations(_ context.Context, _ string, events []eqldb.DropObservation) error {
	if f.err != nil {
		return f.err
	}
	f.drops = append(f.drops, events...)
	return nil
}

func TestRunnerKeepsFailedBatchQueued(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	queue, err := eqldbqueue.Open(filepath.Join(config, "eqdps", "eqldb-queue"))
	if err != nil {
		t.Fatal(err)
	}
	store := eqldb.Store{Path: filepath.Join(config, "eqdps", "eqldb.json")}
	if err := store.Save(eqldb.State{AccessToken: "token"}); err != nil {
		t.Fatal(err)
	}
	if err := queue.Append(eqldbqueue.PlaneOfSky, "sky", skyquest.SyncEvent{
		Character: "Wyrmberg", Server: "rivervale", Type: "quest-turn-in",
		Quest: "bard-test-of-tone", Timestamp: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	runner := &Runner{Client: &fakeAPI{err: errors.New("temporary failure")}, Store: store, Queue: queue}
	if err := runner.upload(context.Background()); err == nil {
		t.Fatal("expected upload failure")
	}
	entries, err := queue.Batch(eqldbqueue.PlaneOfSky, 10)
	if err != nil || len(entries) != 1 {
		t.Fatalf("failed batch was not retained: %#v, %v", entries, err)
	}
}

func TestRunnerUploadsAndAcknowledgesAllQueues(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	queue, err := eqldbqueue.Open(filepath.Join(config, "eqdps", "eqldb-queue"))
	if err != nil {
		t.Fatal(err)
	}
	store := eqldb.Store{Path: filepath.Join(config, "eqdps", "eqldb.json")}
	if err := store.Save(eqldb.State{AccessToken: "token"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "eqdps", "drop-collection-settings.json"), []byte("{\"enabled\":true}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	older := time.Date(2026, 7, 9, 9, 12, 53, 0, time.UTC)
	if err := queue.Append(eqldbqueue.PlaneOfSky, "sky", skyquest.SyncEvent{
		Character: "Wyrmberg", Server: "rivervale", Type: "wind-rune-receive",
		Rune: "Wind Rune Caza", Amount: 1, Timestamp: older,
	}); err != nil {
		t.Fatal(err)
	}
	if err := queue.Append(eqldbqueue.Kills, "kill", dropcollector.Observation{
		Kind: "kill", Zone: "The Plane of Sky", Mob: "a thunder spirit", Timestamp: older,
	}); err != nil {
		t.Fatal(err)
	}
	if err := queue.Append(eqldbqueue.Drops, "drop", dropcollector.Observation{
		Kind: "loot", Zone: "The Plane of Sky", Mob: "a thunder spirit",
		Item: "Wind Rune Caza", Quantity: 2, Timestamp: older.Add(3 * time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	api := new(fakeAPI)
	runner := &Runner{Client: api, Store: store, Queue: queue}
	if err := runner.upload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(api.sky) != 1 || len(api.kills) != 1 || len(api.drops) != 1 || api.drops[0].Amount != 2 {
		t.Fatalf("unexpected uploaded events: sky=%#v kills=%#v drops=%#v", api.sky, api.kills, api.drops)
	}
	for _, name := range []string{eqldbqueue.PlaneOfSky, eqldbqueue.Kills, eqldbqueue.Drops} {
		entries, err := queue.Batch(name, 1)
		if err != nil || len(entries) != 0 {
			t.Fatalf("%s queue was not acknowledged: %#v, %v", name, entries, err)
		}
	}
}

func TestRunnerLeavesObservationsQueuedWhileOptedOut(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	queue, err := eqldbqueue.Open(filepath.Join(config, "eqdps", "eqldb-queue"))
	if err != nil {
		t.Fatal(err)
	}
	store := eqldb.Store{Path: filepath.Join(config, "eqdps", "eqldb.json")}
	if err := store.Save(eqldb.State{AccessToken: "token"}); err != nil {
		t.Fatal(err)
	}
	if err := queue.Append(eqldbqueue.Kills, "kill", dropcollector.Observation{
		Kind: "kill", Zone: "North Ro", Mob: "a zombie", Timestamp: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	api := new(fakeAPI)
	runner := &Runner{Client: api, Store: store, Queue: queue}
	if err := runner.upload(context.Background()); err != nil {
		t.Fatal(err)
	}
	entries, err := queue.Batch(eqldbqueue.Kills, 10)
	if err != nil || len(entries) != 1 || len(api.kills) != 0 {
		t.Fatalf("opted-out observations changed: queued=%d uploaded=%d err=%v", len(entries), len(api.kills), err)
	}
}

func TestRunnerUploadsImmediatelyWhenConnectionTriggersIt(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	queue, err := eqldbqueue.Open(filepath.Join(config, "eqdps", "eqldb-queue"))
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Append(eqldbqueue.PlaneOfSky, "sky", skyquest.SyncEvent{
		Character: "Wyrmberg", Server: "rivervale", Type: "wind-rune-receive",
		Rune: "Wind Rune Caza", Amount: 1, Timestamp: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	store := eqldb.Store{Path: filepath.Join(config, "eqdps", "eqldb.json")}
	api := &signalingAPI{sky: make(chan struct{}, 1)}
	runner := &Runner{Client: api, Store: store, Queue: queue, wake: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runner.Run(ctx)
		close(done)
	}()

	select {
	case <-api.sky:
		t.Fatal("runner uploaded without a connection token")
	case <-time.After(50 * time.Millisecond):
	}
	if err := store.Save(eqldb.State{AccessToken: "new-token"}); err != nil {
		t.Fatal(err)
	}
	runner.Trigger()

	select {
	case <-api.sky:
	case <-time.After(time.Second):
		t.Fatal("triggered runner did not upload the queued event")
	}
	cancel()
	<-done
}
