package target

import (
	"context"
	"time"
)

// Status is the presence status to be pushed to a target.
type Status struct {
	Emoji      string
	Text       string
	Expiration time.Time // zero value means no expiration
}

// Target can sync a user's presence status with an external service.
// Implement this interface to add support for a new target (e.g. Discord).
type Target interface {
	// Sync pushes status to the target.
	// A nil status instructs the target to clear any previously set status.
	Sync(ctx context.Context, status *Status) error
}
