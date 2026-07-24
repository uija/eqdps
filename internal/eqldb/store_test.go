package eqldb

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "eqdps", "eqldb.json")}
	want := State{
		IntroductionShown: true,
		AccessToken:       "private-token",
		ConnectionID:      "connection-id",
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("unexpected state: %#v", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(store.Path)
		if err != nil {
			t.Fatal(err)
		}
		if permissions := info.Mode().Perm(); permissions != 0o600 {
			t.Fatalf("unexpected permissions: %o", permissions)
		}
	}
}

func TestStoreMissingFileIsEmpty(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "missing", "eqldb.json")}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state != (State{}) {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestUploadLeaseIsSharedAndExpires(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "eqdps", "eqldb.json")}
	now := time.Now()
	acquired, _, err := store.AcquireUploadLease(now, 15*time.Second)
	if err != nil || !acquired {
		t.Fatalf("first lease: acquired=%t err=%v", acquired, err)
	}
	acquired, remaining, err := store.AcquireUploadLease(now.Add(5*time.Second), 15*time.Second)
	if err != nil || acquired || remaining <= 0 {
		t.Fatalf("active lease: acquired=%t remaining=%s err=%v", acquired, remaining, err)
	}
	acquired, _, err = store.AcquireUploadLease(now.Add(16*time.Second), 15*time.Second)
	if err != nil || !acquired {
		t.Fatalf("expired lease: acquired=%t err=%v", acquired, err)
	}
}
