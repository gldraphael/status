package availability

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/gldraphael/status/internal/store"
)

// Handler serves availability entries from the stored snapshot.
type Handler struct {
	store                      *store.Store
	apiKey                     string
	blocks                     []Block
	workingHours               WorkingHours
	excludeEnglandBankHolidays bool
	logger                     zerolog.Logger
}

// NewHandler creates a new availability HTTP handler.
func NewHandler(st *store.Store, apiKey string, blocks []Block, workingHours WorkingHours, excludeEnglandBankHolidays bool, logger zerolog.Logger) *Handler {
	return &Handler{
		store:                      st,
		apiKey:                     apiKey,
		blocks:                     blocks,
		workingHours:               workingHours,
		excludeEnglandBankHolidays: excludeEnglandBankHolidays,
		logger:                     logger,
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != h.apiKey {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	snap, ok, err := h.store.GetAvailabilitySnapshot()
	if err != nil {
		h.logger.Error().Err(err).Msg("get availability snapshot")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	opts := ComputeOptions{
		WorkingHours: h.workingHours,
		Now:          time.Now(),
	}
	if h.excludeEnglandBankHolidays {
		holidaySnap, ok, err := h.store.GetHolidaySnapshot()
		if err != nil {
			h.logger.Error().Err(err).Msg("get bank holiday snapshot")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		opts.ExcludeEnglandBankHolidays = true
		opts.HolidayDates = holidaySnap.Dates
	}

	entries, err := Compute(snap.Body, snap.Timezone, h.blocks, opts)
	if err != nil {
		h.logger.Error().Err(err).Msg("compute availability")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		h.logger.Error().Err(err).Msg("encode availability response")
	}
}
