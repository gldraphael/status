package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// Server is a gracefully-shutting-down HTTP server.
type Server struct {
	http   *http.Server
	logger zerolog.Logger
}

// New creates a Server listening on the given port.
func New(port int, handler http.Handler, logger zerolog.Logger) *Server {
	return &Server{
		http: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		logger: logger,
	}
}

// Start starts the server and blocks until ctx is cancelled, then shuts down gracefully.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.http.Addr, err)
	}
	s.logger.Info().Str("addr", ln.Addr().String()).Msg("server listening")

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.http.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.logger.Info().Msg("shutting down server")
		return s.http.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}
