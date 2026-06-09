package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRelayReplacesAuthorizationHeader(t *testing.T) {
	var upstreamAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := defaultConfig()
	cfg.Upstream.BaseURL = upstream.URL
	cfg.Upstream.APIKey = "real-upstream-key"
	cfg.Tokens = nil
	plain := "llmr_client_abc"
	if err := AddToken(&cfg, plain); err != nil {
		t.Fatal(err)
	}

	srv, err := NewRelayServer(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	relay := httptest.NewServer(srv.Handler())
	defer relay.Close()

	req, err := http.NewRequest(http.MethodPost, relay.URL+"/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+plain)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
	expectedAuth := "Bearer " + cfg.Upstream.APIKey
	if upstreamAuth != expectedAuth {
		t.Fatalf("upstream auth mismatch: %q", upstreamAuth)
	}
}

func TestRelayRejectsInvalidToken(t *testing.T) {
	cfg := defaultConfig()
	cfg.Upstream.BaseURL = "http://example.com"
	cfg.Upstream.APIKey = "real-upstream-key"

	srv, err := NewRelayServer(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	relay := httptest.NewServer(srv.Handler())
	defer relay.Close()

	req, err := http.NewRequest(http.MethodPost, relay.URL+"/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "invalid")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
