package availability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/gldraphael/status/internal/feed"
	"github.com/gldraphael/status/internal/store"
)

const englandBankHolidaysURL = "https://www.gov.uk/bank-holidays.json"

// HolidayClient fetches the GOV.UK bank-holidays feed.
type HolidayClient struct {
	url string
}

// NewHolidayClient creates a client for the fixed GOV.UK bank-holidays feed.
func NewHolidayClient() *HolidayClient {
	return &HolidayClient{url: englandBankHolidaysURL}
}

// Fetch returns the raw bank-holidays JSON body.
func (c *HolidayClient) Fetch(ctx context.Context) ([]byte, error) {
	body, err := feed.FetchBody(ctx, c.url, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("fetch bank holidays: %w", err)
	}
	return body, nil
}

// SyncHolidaySnapshot fetches, parses, and stores the holiday snapshot once.
func SyncHolidaySnapshot(ctx context.Context, st *store.Store, client interface {
	Fetch(context.Context) ([]byte, error)
}, logger zerolog.Logger) error {
	body, err := client.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("fetch bank holidays: %w", err)
	}

	dates, err := parseHolidayDates(body)
	if err != nil {
		return fmt.Errorf("parse bank holidays: %w", err)
	}

	snap := &store.HolidaySnapshot{
		Body:      string(body),
		Dates:     dates,
		FetchedAt: time.Now().UTC(),
	}
	if err := st.SetHolidaySnapshot(snap); err != nil {
		return fmt.Errorf("store bank holiday snapshot: %w", err)
	}

	logger.Info().
		Int("dates", len(dates)).
		Time("fetchedAt", snap.FetchedAt).
		Msg("synced bank holidays")

	return nil
}

type bankHolidaysFeed struct {
	EnglandAndWales bankHolidayDivision `json:"england-and-wales"`
}

type bankHolidayDivision struct {
	Division string             `json:"division"`
	Events   []bankHolidayEvent `json:"events"`
}

type bankHolidayEvent struct {
	Date string `json:"date"`
}

func parseHolidayDates(body []byte) ([]string, error) {
	var feed bankHolidaysFeed
	if err := json.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("decode bank holidays: %w", err)
	}

	dates := make([]string, 0, len(feed.EnglandAndWales.Events))
	seen := make(map[string]struct{}, len(feed.EnglandAndWales.Events))
	for i, event := range feed.EnglandAndWales.Events {
		if event.Date == "" {
			return nil, fmt.Errorf("bank holidays events[%d].date is required", i)
		}
		if _, err := time.Parse("2006-01-02", event.Date); err != nil {
			return nil, fmt.Errorf("bank holidays events[%d].date: %w", i, err)
		}
		if _, ok := seen[event.Date]; ok {
			continue
		}
		seen[event.Date] = struct{}{}
		dates = append(dates, event.Date)
	}
	return dates, nil
}
