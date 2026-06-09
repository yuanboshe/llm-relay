package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"version"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(version) returned error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != "dev" {
		t.Fatalf("version output = %q, want dev", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"missing"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(missing) returned nil error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %q, want unknown command", err.Error())
	}
	if !strings.Contains(stderr.String(), "usage: llm-relay") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunServeDryOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"serve", "--addr", "127.0.0.1:9090"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(serve) returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "127.0.0.1:9090") {
		t.Fatalf("serve output = %q, want address", out)
	}
	if !strings.Contains(out, "/v1/chat/completions") {
		t.Fatalf("serve output = %q, want OpenAI route", out)
	}
}
