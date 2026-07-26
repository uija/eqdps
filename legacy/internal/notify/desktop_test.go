package notify

import (
	"context"
	"testing"
)

func TestCancelledNotificationIsSkipped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (Desktop{}).Send(ctx, "title", "message", "", false); err != nil {
		t.Fatal(err)
	}
}
