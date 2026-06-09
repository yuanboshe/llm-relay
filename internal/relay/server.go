package relay

// Server describes the HTTP relay server skeleton.
type Server struct {
	addr string
}

// NewServer constructs a relay server with OpenAI-compatible and Anthropic-compatible route placeholders.
func NewServer(addr string) *Server {
	if addr == "" {
		addr = "127.0.0.1:18080"
	}
	return &Server{addr: addr}
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return s.addr
}

// Routes returns the initial compatibility surface planned for the relay.
func (s *Server) Routes() []string {
	return []string{
		"GET /v1/models",
		"POST /v1/chat/completions",
		"POST /v1/messages",
	}
}
