package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yuanboshe/llm-relay/internal/config"
	"github.com/yuanboshe/llm-relay/internal/tokenstore"
)

func TestNewServerDefaultAddr(t *testing.T) {
	server := NewServer("")
	if server.Addr() != "127.0.0.1:18080" {
		t.Fatalf("Addr = %q, want default", server.Addr())
	}
}

func TestRoutes(t *testing.T) {
	server := NewServer("127.0.0.1:9000")
	routes := server.Routes()

	want := map[string]bool{
		"GET /v1/models":            false,
		"POST /v1/chat/completions": false,
		"POST /v1/messages":         false,
	}
	for _, route := range routes {
		if _, ok := want[route]; ok {
			want[route] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Fatalf("Routes missing %q in %v", route, routes)
		}
	}
}

func TestHandlerRejectsMissingAndInvalidAuthorization(t *testing.T) {
	server := NewProxyServer(Options{
		Addr:   "127.0.0.1:0",
		Config: configuredRelayConfig("https://upstream.example.test/v1", "runtime-key"),
		Tokens: []tokenstore.Record{
			tokenstore.NewRecord("local", "llmr_valid", time.Now()),
		},
	})

	for _, tt := range []struct {
		name          string
		authorization string
		status        int
	}{
		{name: "missing", status: http.StatusUnauthorized},
		{name: "wrong scheme", authorization: "Basic abc", status: http.StatusUnauthorized},
		{name: "wrong prefix", authorization: "Bearer other-token", status: http.StatusUnauthorized},
		{name: "unknown", authorization: "Bearer llmr_unknown", status: http.StatusUnauthorized},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			rec := httptest.NewRecorder()

			server.Handler().ServeHTTP(rec, req)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, tt.status, rec.Body.String())
			}
		})
	}
}

func TestHandlerRejectsDisabledTokenAndDisallowedPath(t *testing.T) {
	tokenValue := "llmr_disabled"
	record := tokenstore.NewRecord("local", tokenValue, time.Now())
	record.Enabled = false
	server := NewProxyServer(Options{
		Addr:   "127.0.0.1:0",
		Config: configuredRelayConfig("https://upstream.example.test/v1", "runtime-key"),
		Tokens: []tokenstore.Record{record},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("disabled status = %d, want 403", rec.Code)
	}

	record.Enabled = true
	server = NewProxyServer(Options{
		Addr:   "127.0.0.1:0",
		Config: configuredRelayConfig("https://upstream.example.test/v1", "runtime-key"),
		Tokens: []tokenstore.Record{record},
	})
	req = httptest.NewRequest(http.MethodGet, "/v1/not-allowed", nil)
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("path status = %d, want 404", rec.Code)
	}
}

func TestHandlerReplacesAuthorizationAndAvoidsDuplicateV1(t *testing.T) {
	tokenValue := "llmr_valid"
	upstreamKey := strings.Join([]string{"runtime", "upstream", "key"}, "-")
	var seenAuth string
	var seenPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	server := NewProxyServer(Options{
		Addr:   "127.0.0.1:0",
		Config: configuredRelayConfig(upstream.URL+"/v1", upstreamKey),
		Tokens: []tokenstore.Record{
			tokenstore.NewRecord("local", tokenValue, time.Now()),
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if seenAuth != "Bearer "+upstreamKey {
		t.Fatalf("Authorization = %q, want upstream key", seenAuth)
	}
	if strings.Contains(seenAuth, tokenValue) {
		t.Fatalf("upstream saw relay token in Authorization")
	}
	if seenPath != "/v1/chat/completions" {
		t.Fatalf("path = %q, want /v1/chat/completions", seenPath)
	}
}

func TestHandlerStreamsSSEWithoutFullBuffering(t *testing.T) {
	tokenValue := "llmr_stream"
	upstreamKey := strings.Join([]string{"runtime", "stream", "key"}, "-")
	firstSent := make(chan struct{})
	sendSecond := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer is not flushable")
		}
		_, _ = w.Write([]byte("data: first\n\n"))
		flusher.Flush()
		close(firstSent)
		<-sendSecond
		_, _ = w.Write([]byte("data: second\n\n"))
		flusher.Flush()
	}))
	defer upstream.Close()

	server := NewProxyServer(Options{
		Addr:   "127.0.0.1:0",
		Config: configuredRelayConfig(upstream.URL, upstreamKey),
		Tokens: []tokenstore.Record{
			tokenstore.NewRecord("local", tokenValue, time.Now()),
		},
	})
	relay := httptest.NewServer(server.Handler())
	defer relay.Close()

	req, err := http.NewRequest(http.MethodPost, relay.URL+"/v1/messages", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("relay request: %v", err)
	}
	defer resp.Body.Close()

	<-firstSent
	buf := make([]byte, len("data: first\n\n"))
	if _, err := io.ReadFull(resp.Body, buf); err != nil {
		t.Fatalf("read first event: %v", err)
	}
	if string(buf) != "data: first\n\n" {
		t.Fatalf("first event = %q", string(buf))
	}
	close(sendSecond)
	rest, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read rest: %v", err)
	}
	if !strings.Contains(string(rest), "data: second") {
		t.Fatalf("rest = %q, want second event", string(rest))
	}
}

func configuredRelayConfig(baseURL string, apiKey string) config.Config {
	cfg := config.DefaultConfig()
	cfg.Upstream.BaseURL = baseURL
	cfg.Upstream.APIKeySource = "inline"
	cfg.Upstream.APIKey = apiKey
	return cfg
}
