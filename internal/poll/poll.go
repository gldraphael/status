package poll

import (
	"context"
	"time"
)

// Every runs fn immediately, then at each interval until ctx is cancelled.
func Every(ctx context.Context, interval time.Duration, fn func() error, onError func(error, bool)) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := fn(); err != nil {
		onError(err, true)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := fn(); err != nil {
				onError(err, false)
			}
		}
	}
}
