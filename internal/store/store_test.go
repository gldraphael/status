package store_test

import (
	"testing"
	"time"

	"github.com/gldraphael/status/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestStatus_SetGetDelete(t *testing.T) {
	st := newTestStore(t)

	// Not found initially.
	_, ok, err := st.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected not found before set")
	}

	want := &store.Status{
		Emoji:      ":calendar:",
		Text:       "Standup",
		Expiration: time.Now().Add(time.Hour).Truncate(time.Millisecond),
	}
	if err := st.SetStatus(want); err != nil {
		t.Fatal(err)
	}

	got, ok, err := st.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected found after set")
	}
	if got.Emoji != want.Emoji || got.Text != want.Text {
		t.Errorf("status mismatch: got %+v, want %+v", got, want)
	}

	// Delete.
	if err := st.DeleteStatus(); err != nil {
		t.Fatal(err)
	}
	_, ok, err = st.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected not found after delete")
	}

	// Double-delete should not error.
	if err := st.DeleteStatus(); err != nil {
		t.Fatalf("double delete: %v", err)
	}
}

func TestAvailabilitySnapshot_SetGet(t *testing.T) {
	st := newTestStore(t)

	_, ok, err := st.GetAvailabilitySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected not found before set")
	}

	want := &store.AvailabilitySnapshot{
		Body:      "BEGIN:VCALENDAR\nEND:VCALENDAR",
		Timezone:  "Europe/London",
		FetchedAt: time.Now().Truncate(time.Millisecond),
	}
	if err := st.SetAvailabilitySnapshot(want); err != nil {
		t.Fatal(err)
	}

	got, ok, err := st.GetAvailabilitySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected found after set")
	}
	if got.Body != want.Body || got.Timezone != want.Timezone {
		t.Errorf("availability snapshot mismatch: got %+v, want %+v", got, want)
	}

	want.Body = "BEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR"
	want.Timezone = "UTC"
	if err := st.SetAvailabilitySnapshot(want); err != nil {
		t.Fatal(err)
	}
	got, _, err = st.GetAvailabilitySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if got.Timezone != "UTC" {
		t.Errorf("expected overwrite to update timezone, got %+v", got)
	}
}

func TestHolidaySnapshot_SetGet(t *testing.T) {
	st := newTestStore(t)

	_, ok, err := st.GetHolidaySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected not found before set")
	}

	want := &store.HolidaySnapshot{
		Body:      `{"england-and-wales":{"division":"england-and-wales","events":[]}}`,
		Dates:     []string{"2026-04-06", "2026-12-25"},
		FetchedAt: time.Now().Truncate(time.Millisecond),
	}
	if err := st.SetHolidaySnapshot(want); err != nil {
		t.Fatal(err)
	}

	got, ok, err := st.GetHolidaySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected found after set")
	}
	if got.Body != want.Body || len(got.Dates) != len(want.Dates) {
		t.Fatalf("holiday snapshot mismatch: got %+v, want %+v", got, want)
	}

	want.Body = `{"england-and-wales":{"division":"england-and-wales","events":[{"date":"2026-04-07"}]}}`
	want.Dates = []string{"2026-04-07"}
	if err := st.SetHolidaySnapshot(want); err != nil {
		t.Fatal(err)
	}
	got, _, err = st.GetHolidaySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Dates) != 1 || got.Dates[0] != "2026-04-07" {
		t.Errorf("expected overwrite to update dates, got %+v", got)
	}
}

func TestAvailabilityDeploymentState_SetGetClear(t *testing.T) {
	st := newTestStore(t)

	_, ok, err := st.GetAvailabilityDirty()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected dirty state not found before set")
	}

	dirty := []byte(`[{"date":"2026-04-06","block":"Morning"}]`)
	if err := st.SetAvailabilityDirty(dirty); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetAvailabilityDirty()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected dirty state after set")
	}
	if string(got) != string(dirty) {
		t.Fatalf("dirty mismatch: got %s, want %s", got, dirty)
	}

	if err := st.SetLastDeployedAvailability(got); err != nil {
		t.Fatal(err)
	}
	last, ok, err := st.GetLastDeployedAvailability()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected last deployed state after set")
	}
	if string(last) != string(dirty) {
		t.Fatalf("last deployed mismatch: got %s, want %s", last, dirty)
	}

	if err := st.ClearAvailabilityDirty(); err != nil {
		t.Fatal(err)
	}
	_, ok, err = st.GetAvailabilityDirty()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected dirty state to be cleared")
	}
}

func TestEvent_SetGet(t *testing.T) {
	st := newTestStore(t)

	_, ok, err := st.GetEvent("evt1")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected not found before set")
	}

	want := &store.Event{
		ID:        "evt1",
		Summary:   "Team meeting",
		StartTime: time.Now().Add(-30 * time.Minute).Truncate(time.Millisecond),
		EndTime:   time.Now().Add(30 * time.Minute).Truncate(time.Millisecond),
		Cancelled: false,
	}
	if err := st.SetEvent(want); err != nil {
		t.Fatal(err)
	}

	got, ok, err := st.GetEvent("evt1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected found after set")
	}
	if got.ID != want.ID || got.Summary != want.Summary || got.Cancelled != want.Cancelled {
		t.Errorf("event mismatch: got %+v, want %+v", got, want)
	}

	// Overwrite: mark as cancelled.
	want.Cancelled = true
	if err := st.SetEvent(want); err != nil {
		t.Fatal(err)
	}
	got, _, err = st.GetEvent("evt1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Cancelled {
		t.Error("expected event to be cancelled after update")
	}
}

func TestListActiveEvents(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()

	tests := []struct {
		ev     store.Event
		active bool // expected to appear in active events at `now`
	}{
		{
			ev: store.Event{
				ID: "past", Summary: "Past meeting",
				StartTime: now.Add(-2 * time.Hour), EndTime: now.Add(-time.Hour),
			},
			active: false,
		},
		{
			ev: store.Event{
				ID: "current", Summary: "Current meeting",
				StartTime: now.Add(-30 * time.Minute), EndTime: now.Add(30 * time.Minute),
			},
			active: true,
		},
		{
			ev: store.Event{
				ID: "future", Summary: "Future meeting",
				StartTime: now.Add(time.Hour), EndTime: now.Add(2 * time.Hour),
			},
			active: false,
		},
		{
			// Cancelled events should never be active.
			ev: store.Event{
				ID: "cancelled", Summary: "Cancelled meeting",
				StartTime: now.Add(-30 * time.Minute), EndTime: now.Add(30 * time.Minute),
				Cancelled: true,
			},
			active: false,
		},
		{
			// Event that starts exactly now is active.
			ev: store.Event{
				ID: "starts-now", Summary: "Starts now",
				StartTime: now, EndTime: now.Add(time.Hour),
			},
			active: true,
		},
		{
			// Multi-day event.
			ev: store.Event{
				ID: "multi-day", Summary: "Multi-day workshop",
				StartTime: now.Add(-24 * time.Hour), EndTime: now.Add(24 * time.Hour),
			},
			active: true,
		},
	}

	for i := range tests {
		if err := st.SetEvent(&tests[i].ev); err != nil {
			t.Fatalf("set event %q: %v", tests[i].ev.ID, err)
		}
	}

	active, err := st.ListActiveEvents(now)
	if err != nil {
		t.Fatal(err)
	}

	activeIDs := make(map[string]bool, len(active))
	for _, ev := range active {
		activeIDs[ev.ID] = true
	}

	for _, tc := range tests {
		got := activeIDs[tc.ev.ID]
		if got != tc.active {
			t.Errorf("event %q: active=%v, want active=%v", tc.ev.ID, got, tc.active)
		}
	}
}

func TestListActiveEvents_Empty(t *testing.T) {
	st := newTestStore(t)
	active, err := st.ListActiveEvents(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active events, got %d", len(active))
	}
}
