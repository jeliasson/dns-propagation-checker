// Package server exposes the propagation checker over HTTP: a JSON API plus
// the embedded single-page frontend.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jeliasson/dns-propagation-checker/internal/resolve"
	"github.com/jeliasson/dns-propagation-checker/web"
)

type Config struct {
	// RateBurst requests are allowed per RateWindow, per client.
	RateBurst  int
	RateWindow time.Duration

	// QueryTimeout bounds each individual DNS query.
	QueryTimeout time.Duration

	// TrustProxy makes the rate limiter read the client address from
	// X-Forwarded-For. Only enable this behind a proxy that sets the header,
	// otherwise clients can spoof it to bypass the limit.
	TrustProxy bool

	Logger *slog.Logger
}

// DefaultConfig returns the limits used when nothing is configured.
func DefaultConfig() Config {
	return Config{
		RateBurst:    20,
		RateWindow:   time.Minute,
		QueryTimeout: resolve.DefaultTimeout,
		Logger:       slog.Default(),
	}
}

// Server is an http.Handler serving the UI and the API.
type Server struct {
	cfg     Config
	log     *slog.Logger
	limit   *limiter
	mux     *http.ServeMux
	done    chan struct{}
	maxBody int64
}

// New builds a Server. Call Close to stop its background sweeper.
func New(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.QueryTimeout <= 0 {
		cfg.QueryTimeout = resolve.DefaultTimeout
	}

	s := &Server{
		cfg:     cfg,
		log:     cfg.Logger,
		limit:   newLimiter(cfg.RateBurst, cfg.RateWindow),
		mux:     http.NewServeMux(),
		done:    make(chan struct{}),
		maxBody: 64 << 10,
	}
	s.routes()
	go s.limit.sweepEvery(cfg.RateWindow, s.done)
	return s
}

// Close releases background resources.
func (s *Server) Close() {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /api/query", s.rateLimited(s.handleQuery))
	s.mux.HandleFunc("GET /api/defaults", s.handleDefaults)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, "ok\n")
	})
	s.mux.Handle("GET /", web.Handler())
}

// queryRequest is the POST /api/query body.
type queryRequest struct {
	Servers []resolve.Server `json:"servers"`
	Records []resolve.Record `json:"records"`
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req queryRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, s.maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.QueryTimeout+5*time.Second)
	defer cancel()

	result, err := resolve.Do(ctx, resolve.Request{
		Servers: req.Servers,
		Records: req.Records,
		Timeout: s.cfg.QueryTimeout,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.log.Info("query",
		"records", len(result.Records),
		"servers", len(req.Servers),
		"diff", result.Diff,
		"duration_ms", result.DurationMS,
	)
	writeJSON(w, http.StatusOK, result)
}

// handleDefaults gives the frontend its starting state without hardcoding it
// in two places.
func (s *Server) handleDefaults(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"servers":      resolve.DefaultServers,
		"record_types": resolve.RecordTypes,
		"records": []resolve.Record{
			{FQDN: "github.com", Type: "NS"},
			{FQDN: "github.com", Type: "MX"},
			{FQDN: "github.com", Type: "A"},
		},
		"limits": map[string]int{
			"max_servers": resolve.MaxServers,
			"max_records": resolve.MaxRecords,
		},
	})
}

// rateLimited rejects callers over their request budget with 429.
func (s *Server) rateLimited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limit.allow(s.clientKey(r)) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded, slow down")
			return
		}
		next(w, r)
	}
}

// clientKey identifies the caller for rate limiting.
func (s *Server) clientKey(r *http.Request) string {
	if s.cfg.TrustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if first, _, found := strings.Cut(fwd, ","); found {
				return strings.TrimSpace(first)
			}
			return strings.TrimSpace(fwd)
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Debug("write response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
