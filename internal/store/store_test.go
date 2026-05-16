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

func TestStatusRawSnapshot_SetGet(t *testing.T) {
	st := newTestStore(t)

	_, ok, err := st.GetStatusRawSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected not found before set")
	}

	want := &store.CalendarSnapshot{
		Body:      "BEGIN:VCALENDAR\nEND:VCALENDAR",
		Timezone:  "Europe/London",
		FetchedAt: time.Now().Truncate(time.Millisecond),
	}
	if err := st.SetStatusRawSnapshot(want); err != nil {
		t.Fatal(err)
	}

	got, ok, err := st.GetStatusRawSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected found after set")
	}
	if got.Body != want.Body || got.Timezone != want.Timezone {
		t.Errorf("status raw snapshot mismatch: got %+v, want %+v", got, want)
	}
}

func TestStatusProjection_Replace(t *testing.T) {
	st := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)
	raw := &store.CalendarSnapshot{
		Body:      "BEGIN:VCALENDAR\nEND:VCALENDAR",
		Timezone:  "Europe/London",
		FetchedAt: now,
	}
	current := &store.Status{
		Emoji:      ":calendar:",
		Text:       "Team meeting",
		Expiration: now.Add(30 * time.Minute),
	}

	if err := st.ReplaceStatusProjection(raw, current); err != nil {
		t.Fatal(err)
	}

	gotRaw, ok, err := st.GetStatusRawSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected status raw snapshot after replace")
	}
	if gotRaw.Body != raw.Body || gotRaw.Timezone != raw.Timezone {
		t.Fatalf("unexpected status raw snapshot: %+v", gotRaw)
	}

	gotStatus, ok, err := st.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected current status after replace")
	}
	if gotStatus.Text != current.Text {
		t.Fatalf("status text: got %q, want %q", gotStatus.Text, current.Text)
	}

	raw.Body = "BEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR"
	if err := st.ReplaceStatusProjection(raw, nil); err != nil {
		t.Fatal(err)
	}
	_, ok, err = st.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected current status to be deleted")
	}
}

func TestAvailabilityRawSnapshot_SetGet(t *testing.T) {
	st := newTestStore(t)

	_, ok, err := st.GetAvailabilityRawSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected not found before set")
	}

	want := &store.CalendarSnapshot{
		Body:      "BEGIN:VCALENDAR\nEND:VCALENDAR",
		Timezone:  "Europe/London",
		FetchedAt: time.Now().Truncate(time.Millisecond),
	}
	if err := st.SetAvailabilityRawSnapshot(want); err != nil {
		t.Fatal(err)
	}

	got, ok, err := st.GetAvailabilityRawSnapshot()
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
	if err := st.SetAvailabilityRawSnapshot(want); err != nil {
		t.Fatal(err)
	}
	got, _, err = st.GetAvailabilityRawSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if got.Timezone != "UTC" {
		t.Errorf("expected overwrite to update timezone, got %+v", got)
	}
}

func TestAvailabilityCurrent_SetGetClear(t *testing.T) {
	st := newTestStore(t)

	_, ok, err := st.GetAvailabilityCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected current availability not found before set")
	}

	current := []byte(`[{"day_of_week":"Monday","block":"Morning","date":"2026-04-06"}]`)
	if err := st.SetAvailabilityCurrent(current); err != nil {
		t.Fatal(err)
	}

	got, ok, err := st.GetAvailabilityCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected current availability after set")
	}
	if string(got) != string(current) {
		t.Fatalf("current availability mismatch: got %s, want %s", got, current)
	}

	if err := st.ClearAvailabilityCurrent(); err != nil {
		t.Fatal(err)
	}
	_, ok, err = st.GetAvailabilityCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected current availability to be cleared")
	}
}

func TestAvailabilityProjection_Replace(t *testing.T) {
	st := newTestStore(t)
	raw := &store.CalendarSnapshot{
		Body:      "BEGIN:VCALENDAR\nEND:VCALENDAR",
		Timezone:  "Europe/London",
		FetchedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	current := []byte(`[{"date":"2026-04-06","block":"Morning"}]`)

	if err := st.ReplaceAvailabilityProjection(raw, current); err != nil {
		t.Fatal(err)
	}

	gotRaw, ok, err := st.GetAvailabilityRawSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected availability raw snapshot")
	}
	if gotRaw.Body != raw.Body || gotRaw.Timezone != raw.Timezone {
		t.Fatalf("unexpected raw snapshot: %+v", gotRaw)
	}

	gotCurrent, ok, err := st.GetAvailabilityCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected current availability")
	}
	if string(gotCurrent) != string(current) {
		t.Fatalf("current availability mismatch: got %s, want %s", gotCurrent, current)
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

	dirty = []byte(`[{"date":"2026-04-07","block":"Afternoon"}]`)
	if err := st.SetAvailabilityDirty(dirty); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkAvailabilityDeployed(dirty); err != nil {
		t.Fatal(err)
	}
	last, ok, err = st.GetLastDeployedAvailability()
	if err != nil {
		t.Fatal(err)
	}
	if !ok || string(last) != string(dirty) {
		t.Fatalf("last deployed mismatch after mark deployed: got %s ok=%v", last, ok)
	}
	_, ok, err = st.GetAvailabilityDirty()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected dirty state to be cleared after mark deployed")
	}
}
