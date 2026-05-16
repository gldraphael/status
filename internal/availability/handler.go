package availability

import (
	"errors"
	"net/http"

	"github.com/rs/zerolog"
)

// Handler serves precomputed availability entries from the store.
type Handler struct {
	provider *Provider
	apiKey   string
	logger   zerolog.Logger
}

// NewHandler creates a new availability HTTP handler.
func NewHandler(provider *Provider, apiKey string, logger zerolog.Logger) *Handler {
	return &Handler{
		provider: provider,
		apiKey:   apiKey,
		logger:   logger,
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != h.apiKey {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	data, err := h.provider.GetEntriesJSON()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		h.logger.Error().Err(err).Msg("get availability entries")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(data); err != nil {
		h.logger.Error().Err(err).Msg("write availability response")
	}
}
