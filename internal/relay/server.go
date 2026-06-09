package relay

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/yuanboshe/llm-relay/internal/config"
	"github.com/yuanboshe/llm-relay/internal/tokenstore"
)

// Options configures the HTTP relay server.
type Options struct {
	Addr   string
	Config config.Config
	Tokens []tokenstore.Record
	Client *http.Client
}

// Server describes the HTTP relay server.
type Server struct {
	addr   string
	cfg    config.Config
	tokens []tokenstore.Record
	client *http.Client
}

// NewServer constructs a relay server with OpenAI-compatible and Anthropic-compatible route placeholders.
func NewServer(addr string) *Server {
	if addr == "" {
		addr = "127.0.0.1:18080"
	}
	return &Server{addr: addr}
}

// NewProxyServer constructs a relay server that validates relay tokens and proxies upstream requests.
func NewProxyServer(opts Options) *Server {
	addr := opts.Addr
	if addr == "" {
		addr = opts.Config.ListenAddr
	}
	if addr == "" {
		addr = "127.0.0.1:18080"
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 0}
	}
	return &Server{
		addr:   addr,
		cfg:    opts.Config,
		tokens: opts.Tokens,
		client: client,
	}
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return s.addr
}

// Routes returns the initial compatibility surface planned for the relay.
func (s *Server) Routes() []string {
	return []string{
		"GET /v1/models",
		"POST /v1/responses",
		"POST /v1/completions",
		"POST /v1/embeddings",
		"POST /v1/chat/completions",
		"POST /v1/messages",
	}
}

// Handler returns the HTTP handler for the relay.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.handle)
}

// ListenAndServe starts the relay server until the context is canceled or the listener fails.
func (s *Server) ListenAndServe(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	httpServer := &http.Server{Handler: s.Handler()}
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		_ = httpServer.Close()
		err := <-errCh
		if err == http.ErrServerClosed {
			return ctx.Err()
		}
		return err
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	record, ok := s.authenticate(r.Header.Get("Authorization"))
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !record.Enabled {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !isAllowedPath(r.URL.Path) {
		http.NotFound(w, r)
		return
	}

	target, err := upstreamURL(s.cfg.Upstream.BaseURL, r.URL)
	if err != nil {
		http.Error(w, "invalid upstream", http.StatusBadGateway)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadGateway)
		return
	}
	req.Header = r.Header.Clone()
	req.Header.Set("Authorization", "Bearer "+s.cfg.Upstream.APIKey)
	req.Host = ""

	resp, err := s.client.Do(req)
	if err != nil {
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = copyResponse(w, resp.Body)
	_ = record
}

func (s *Server) authenticate(header string) (tokenstore.Record, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || !strings.HasPrefix(token, "llmr_") {
		return tokenstore.Record{}, false
	}
	for _, record := range s.tokens {
		if tokenstore.RecordMatchesToken(record, token) {
			return record, true
		}
	}
	return tokenstore.Record{}, false
}

func isAllowedPath(path string) bool {
	switch path {
	case "/v1/messages",
		"/v1/models",
		"/v1/responses",
		"/v1/chat/completions",
		"/v1/completions",
		"/v1/embeddings":
		return true
	default:
		return false
	}
}

func upstreamURL(baseURL string, requestURL *url.URL) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid base URL")
	}
	requestPath := requestURL.Path
	basePath := strings.TrimRight(base.Path, "/")
	if basePath != "" && strings.HasPrefix(requestPath, basePath+"/") {
		base.Path = requestPath
	} else {
		base.Path = basePath + requestPath
	}
	base.RawQuery = requestURL.RawQuery
	base.Fragment = ""
	return base.String(), nil
}

func copyHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func copyResponse(w http.ResponseWriter, body io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			writeN, writeErr := w.Write(buf[:n])
			written += int64(writeN)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			if writeErr != nil {
				return written, writeErr
			}
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}
