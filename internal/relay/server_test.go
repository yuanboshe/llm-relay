package relay

import "testing"

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
