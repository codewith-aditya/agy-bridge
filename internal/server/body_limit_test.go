package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agy-bridge/internal/config"
)

// A body over the cap must be refused before it is buffered, and the caller has
// to learn why: 413 with the configured size, not a generic read failure.
func TestOversizedBodyIsRefused(t *testing.T) {
	s := New(config.Config{
		UpstreamBase:    "http://127.0.0.1:1",
		DefaultModel:    "test-model",
		MaxRequestBytes: 1 << 10,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1internal:cascadeNuxes", strings.NewReader(strings.Repeat("a", 1<<12)))
	rec := httptest.NewRecorder()

	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "MiB") {
		t.Fatalf("body %q does not name the limit", rec.Body.String())
	}
}

// The cap has to survive a setting that would otherwise disable it.
func TestMaxRequestBytesRefusesZero(t *testing.T) {
	t.Setenv("MAX_REQUEST_MB", "0")
	if got := config.Load().MaxRequestBytes; got != 1<<20 {
		t.Fatalf("a zero MiB cap became %d bytes, want one MiB", got)
	}
}
