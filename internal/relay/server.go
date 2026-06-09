package relay

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var allowedPaths = map[string]bool{
	"/v1/messages":         true,
	"/v1/models":           true,
	"/v1/chat/completions": true,
	"/v1/responses":        true,
	"/v1/completions":      true,
	"/v1/embeddings":       true,
}

type Logger struct {
	mu   sync.Mutex
	file *os.File
}

func NewLogger(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &Logger{file: f}, nil
}

func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

func (l *Logger) Log(entry map[string]any) {
	if l == nil {
		return
	}
	entry["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(entry)
	if err != nil {
		b = []byte(fmt.Sprintf(`{"level":"error","msg":"log_marshal_failed","error":%q}`, err.Error()))
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.file.Write(append(b, '\n'))
}

type RelayServer struct {
	cfg      Config
	client   *http.Client
	upstream *url.URL
	logger   *Logger
}

func NewRelayServer(cfg Config, logger *Logger) (*RelayServer, error) {
	base, err := url.Parse(cfg.Upstream.BaseURL)
	if err != nil {
		return nil, err
	}
	return &RelayServer{
		cfg:      cfg,
		client:   &http.Client{},
		upstream: base,
		logger:   logger,
	}, nil
}

func (s *RelayServer) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		status := http.StatusOK
		defer func() {
			s.logger.Log(map[string]any{
				"level":       "info",
				"msg":         "request",
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      status,
				"duration_ms": time.Since(start).Milliseconds(),
				"client":      r.RemoteAddr,
			})
		}()

		if !allowedPaths[r.URL.Path] {
			status = http.StatusNotFound
			http.NotFound(w, r)
			return
		}

		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(auth, "Bearer ") {
			status = http.StatusUnauthorized
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		clientToken := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if clientToken == "" || !strings.HasPrefix(clientToken, "llmr_") || !HasToken(s.cfg, clientToken) {
			status = http.StatusUnauthorized
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if s.cfg.Upstream.APIKey == "" {
			status = http.StatusBadGateway
			http.Error(w, "upstream API key not configured", http.StatusBadGateway)
			return
		}

		u := *s.upstream
		u.Path = strings.TrimRight(s.upstream.Path, "/") + r.URL.Path
		u.RawQuery = r.URL.RawQuery

		req, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), r.Body)
		if err != nil {
			status = http.StatusBadGateway
			http.Error(w, "proxy request failed", http.StatusBadGateway)
			return
		}
		copyHeaders(req.Header, r.Header)
		req.Header.Set("Authorization", "Bearer "+s.cfg.Upstream.APIKey)
		if strings.EqualFold(s.cfg.Upstream.Provider, "anthropic") {
			req.Header.Set("x-api-key", s.cfg.Upstream.APIKey)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			status = http.StatusBadGateway
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, vals := range resp.Header {
			if strings.EqualFold(k, "content-length") {
				continue
			}
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		status = resp.StatusCode
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(flushWriter{w}, resp.Body)
	})
}

func copyHeaders(dst, src http.Header) {
	for k, vals := range src {
		if strings.EqualFold(k, "Authorization") {
			continue
		}
		for _, v := range vals {
			dst.Add(k, v)
		}
	}
}

type flushWriter struct {
	w http.ResponseWriter
}

func (fw flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return n, err
}

func TailLogs(path string, maxLines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	s := bufio.NewScanner(f)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if maxLines <= 0 || len(lines) <= maxLines {
		return lines, nil
	}
	return lines[len(lines)-maxLines:], nil
}

func WaitForServer(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("server did not start in time")
}
