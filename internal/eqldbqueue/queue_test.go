package eqldbqueue

import (
	"encoding/json"
	"os"
	"testing"
)

func TestQueueAcknowledgesBatchesAndTruncatesWhenDrained(t *testing.T) {
	queue, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two", "three"} {
		if err := queue.Append(Kills, id, map[string]string{"value": id}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := queue.Batch(Kills, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || first[0].ID != "one" || first[1].ID != "two" {
		t.Fatalf("unexpected first batch: %#v", first)
	}
	if err := queue.Acknowledge(Kills, first[1].EndOffset); err != nil {
		t.Fatal(err)
	}
	second, err := queue.Batch(Kills, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].ID != "three" {
		t.Fatalf("unexpected second batch: %#v", second)
	}
	var payload map[string]string
	if err := json.Unmarshal(second[0].Payload, &payload); err != nil || payload["value"] != "three" {
		t.Fatalf("unexpected payload: %#v, %v", payload, err)
	}
	if err := queue.Acknowledge(Kills, second[0].EndOffset); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(queue.Path(Kills))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("drained queue retained %d bytes", info.Size())
	}
}
