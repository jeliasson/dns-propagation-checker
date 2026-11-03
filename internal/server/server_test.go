package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := DefaultConfig()
	cfg.QueryTimeout = 500 * time.Millisecond
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(cfg)
	t.Cleanup(s.Close)
	return s
}

func do(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestIndexIsServedFromTheBinary(t *testing.T) {
	s := newTestServer(t)

	rec := do(t, s, http.MethodGet, "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "DNS Propagation Checker") {
		t.Error("index.html did not come back")
	}

	for _, asset := range []string{"/app.js", "/style.css", "/favicon.svg"} {
		if rec := do(t, s, http.MethodGet, asset, ""); rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d", asset, rec.Code)
		}
	}
	if rec := do(t, s, http.MethodGet, "/nope.txt", ""); rec.Code != http.StatusNotFound {
		t.Errorf("GET /nope.txt = %d, want 404", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	if rec := do(t, newTestServer(t), http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz = %d", rec.Code)
	}
}

func TestDefaultsEndpoint(t *testing.T) {
	rec := do(t, newTestServer(t), http.MethodGet, "/api/defaults", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/defaults = %d", rec.Code)
	}

	var body struct {
		Servers     []map[string]string `json:"servers"`
		RecordTypes []string            `json:"record_types"`
		Records     []map[string]string `json:"records"`
		Limits      map[string]int      `json:"limits"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Servers) == 0 || len(body.RecordTypes) == 0 || len(body.Records) == 0 {
		t.Errorf("defaults are incomplete: %+v", body)
	}
	if body.Limits["max_servers"] == 0 || body.Limits["max_records"] == 0 {
		t.Errorf("limits missing: %+v", body.Limits)
	}
}

func TestQueryRejectsBadRequests(t *testing.T) {
	s := newTestServer(t)

	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "not json", body: "{", want: http.StatusBadRequest},
		{name: "unknown field", body: `{"nope":1}`, want: http.StatusBadRequest},
		{name: "no records", body: `{"servers":[{"address":"1.1.1.1"}]}`, want: http.StatusBadRequest},
		{
			name: "injection attempt in name",
			body: `{"records":[{"fqdn":"a;rm -rf /","type":"A"}]}`,
			want: http.StatusBadRequest,
		},
		{
			name: "injection attempt in server",
			body: `{"records":[{"fqdn":"example.com","type":"A"}],"servers":[{"address":"$(id)"}]}`,
			want: http.StatusBadRequest,
		},
		{
			name: "unsupported type",
			body: `{"records":[{"fqdn":"example.com","type":"ANY"}]}`,
			want: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		rec := do(t, s, http.MethodPost, "/api/query", tc.body)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d (%s)", tc.name, rec.Code, tc.want, rec.Body.String())
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] == "" {
			t.Errorf("%s: expected a JSON error body, got %q", tc.name, rec.Body.String())
		}
	}
}

func TestQueryRejectsOversizedBody(t *testing.T) {
	s := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/query", `{"records":[`+strings.Repeat("x", 100<<10)+`]}`)
	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 413 or 400", rec.Code)
	}
}

func TestQueryRejectsWrongMethod(t *testing.T) {
	// GET /api/query falls through to the static handler, which has no such
	// file; what matters is that it is not treated as a query.
	rec := do(t, newTestServer(t), http.MethodGet, "/api/query", "")
	if rec.Code == http.StatusOK {
		t.Error("GET /api/query should not succeed")
	}
}

func TestQueryReturnsResultsForUnreachableServer(t *testing.T) {
	s := newTestServer(t)

	// Port 1 on loopback will not answer; the request should still succeed
	// with the failure recorded per answer.
	rec := do(t, s, http.MethodPost, "/api/query",
		`{"servers":[{"name":"dead","address":"127.0.0.1:1"}],"records":[{"fqdn":"example.com","type":"A"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}

	var result struct {
		Diff    bool `json:"diff"`
		Records []struct {
			Diff    bool `json:"diff"`
			Answers []struct {
				Error string `json:"error"`
			} `json:"answers"`
		} `json:"records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Records) != 1 || len(result.Records[0].Answers) != 1 {
		t.Fatalf("unexpected shape: %+v", result)
	}
	if result.Records[0].Answers[0].Error == "" {
		t.Error("expected the unreachable server's failure to be reported")
	}
}

func TestRateLimitReturns429(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RateBurst = 2
	cfg.RateWindow = time.Minute
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(cfg)
	defer s.Close()

	// Invalid bodies still consume budget, which keeps the limiter cheap and
	// means a hostile caller cannot get free attempts.
	body := `{"records":[{"fqdn":"example.com","type":"NOPE"}]}`
	for i := range 2 {
		if rec := do(t, s, http.MethodPost, "/api/query", body); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d was limited too early", i+1)
		}
	}

	rec := do(t, s, http.MethodPost, "/api/query", body)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 should carry Retry-After")
	}
}

func TestClientKeyIgnoresForwardedHeaderUnlessTrusted(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/query", nil)
	req.RemoteAddr = "203.0.113.9:4321"
	req.Header.Set("X-Forwarded-For", "198.51.100.1, 10.0.0.1")

	untrusted := New(DefaultConfig())
	defer untrusted.Close()
	if got := untrusted.clientKey(req); got != "203.0.113.9" {
		t.Errorf("untrusted clientKey = %q, want the socket address", got)
	}

	cfg := DefaultConfig()
	cfg.TrustProxy = true
	trusted := New(cfg)
	defer trusted.Close()
	if got := trusted.clientKey(req); got != "198.51.100.1" {
		t.Errorf("trusted clientKey = %q, want the first forwarded address", got)
	}
}

func TestClientKeyHandlesMalformedRemoteAddr(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/query", nil)
	req.RemoteAddr = "weird"
	if got := s.clientKey(req); got != "weird" {
		t.Errorf("clientKey = %q", got)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	s := New(DefaultConfig())
	s.Close()
	s.Close() // must not panic on a double close
}
