package notify

import (
	"context"

	"github.com/gen2brain/beeep"
)

type Desktop struct{}

func init() {
	beeep.AppName = "eqdps"
}

func (Desktop) Send(ctx context.Context, title, message, icon string, requestPersistence bool) error {
	select {
	case <-ctx.Done():
		return nil
	default:
	}
	// Desktop environments control notification duration. beeep does not expose
	// a portable persistence request, so requestPersistence is intentionally a
	// best-effort preference retained in the shared event model.
	_ = requestPersistence
	return beeep.Notify(title, message, icon)
}
