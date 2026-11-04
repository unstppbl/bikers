// Package workers owns actual background polling loops.
package workers

import (
	"context"
	"log"
	"time"
)

type PushDispatcher interface{ Dispatch(context.Context) error }

func RunPush(ctx context.Context, dispatcher PushDispatcher) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := dispatcher.Dispatch(ctx); err != nil && ctx.Err() == nil {
			log.Print("push_dispatch_failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
