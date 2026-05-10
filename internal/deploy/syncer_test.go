package deploy

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/gldraphael/status/internal/store"
)

type mockClient struct {
	triggered bool
	err       error
}

func (m *mockClient) Trigger(_ context.Context) error {
	m.triggered = true
	return m.err
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestDeployer_TriggerIfDirty(t *testing.T) {
	st := newTestStore(t)
	client := &mockClient{}
	d := NewDeployer(client, st, zerolog.Nop())

	t.Run("skips when not dirty", func(t *testing.T) {
		client.triggered = false
		if err := st.SetAvailabilityDirty(false); err != nil {
			t.Fatalf("SetAvailabilityDirty: %v", err)
		}

		d.triggerIfDirty(context.Background(), time.Now())

		if client.triggered {
			t.Fatal("expected deploy NOT to be triggered")
		}
	})

	t.Run("triggers and clears when dirty", func(t *testing.T) {
		client.triggered = false
		if err := st.SetAvailabilityDirty(true); err != nil {
			t.Fatalf("SetAvailabilityDirty: %v", err)
		}

		d.triggerIfDirty(context.Background(), time.Now())

		if !client.triggered {
			t.Fatal("expected deploy to be triggered")
		}

		dirty, err := st.IsAvailabilityDirty()
		if err != nil {
			t.Fatalf("IsAvailabilityDirty: %v", err)
		}
		if dirty {
			t.Error("expected dirty flag to be cleared after successful deploy")
		}
	})

	t.Run("preserves dirty on trigger error", func(t *testing.T) {
		client.triggered = false
		client.err = context.DeadlineExceeded
		if err := st.SetAvailabilityDirty(true); err != nil {
			t.Fatalf("SetAvailabilityDirty: %v", err)
		}

		d.triggerIfDirty(context.Background(), time.Now())

		if !client.triggered {
			t.Fatal("expected deploy to be attempted")
		}

		dirty, err := st.IsAvailabilityDirty()
		if err != nil {
			t.Fatalf("IsAvailabilityDirty: %v", err)
		}
		if !dirty {
			t.Error("expected dirty flag to remain set after failed deploy")
		}
	})
}
