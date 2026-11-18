package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agy-bridge/internal/config"
)

// A client that disconnects must take the upstream call with it: the handler
// used to post without the request context, so the turn finished and billed
// after the caller had already walked away.
func TestPostStopsWhenTheCallerCancels(t *testing.T) {
	reached := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(reached)
		time.Sleep(2 * time.Second)
	}))
	defer upstream.Close()

	s := New(config.Config{UpstreamBase: upstream.URL, DefaultModel: "test-model"})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-reached
		cancel()
	}()

	start := time.Now()
	if _, err := s.post(ctx, "key", []byte(`{"model":"test-model"}`), false); err == nil {
		t.Fatal("a canceled request returned no error")
	}
	if waited := time.Since(start); waited > time.Second {
		t.Fatalf("the canceled request waited %s for the upstream", waited)
	}
}
