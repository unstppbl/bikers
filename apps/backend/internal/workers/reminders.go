package workers

import (
	"context"
	"log"
	"time"
)

type ReminderDispatcher interface{ Dispatch(context.Context) error }

func RunReminders(ctx context.Context, dispatcher ReminderDispatcher) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		if err := dispatcher.Dispatch(ctx); err != nil && ctx.Err() == nil {
			log.Print("ride_reminder_dispatch_failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
