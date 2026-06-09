# llm-relay

`llm-relay` is a Go-based LLM API relay and proxy. It is intended to run locally or on a server and forward compatible requests to configured upstream providers.

Planned compatibility:

- OpenAI-compatible API endpoints.
- Anthropic-compatible API endpoints.
- Provider configuration with `base_url` and API key references.
- Local relay credential generation and lookup.

The default configuration directory is:

```text
~/.llmrelay
```

Suggested files:

```text
~/.llmrelay/config.toml
~/.llmrelay/tokens.json
```

## Quick Start

Build and run the current CLI skeleton:

```sh
go build -o ./llm-relay ./cmd/llm-relay
./llm-relay version
./llm-relay config path
```

Run tests:

```sh
go test ./...
```

## Example Configuration

Use placeholders in committed examples. Keep real keys in your shell, secret manager, or local untracked files.

```toml
listen_addr = "127.0.0.1:8080"

[[providers]]
name = "openai"
kind = "openai"
base_url = "https://api.openai.com/v1"
api_key = "<provider-api-key>"

[[providers]]
name = "anthropic"
kind = "anthropic"
base_url = "https://api.anthropic.com"
api_key = "<provider-api-key>"
```

## CLI Plan

The CLI is intentionally small in this initialization:

- `llm-relay version`
- `llm-relay config path`
- `llm-relay serve`

Planned commands include:

- `llm-relay init`
- `llm-relay token create`
- `llm-relay token list`
- `llm-relay provider validate`

## Security

Do not commit real API keys, relay credentials, deployment URLs, or local configuration files. Use `.env.example` and `examples/` only for placeholders.

