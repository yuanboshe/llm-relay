# llm-relay

`llm-relay` is an early Go skeleton for a local or server-side LLM API relay. Its command-line binary is `llmrelay`.

The current repository contains foundational local configuration, relay token management, single-upstream configuration, diagnostic commands, and local install support. It does not yet implement a production relay, request forwarding, usage tracking, or full OpenAI-compatible / Anthropic-compatible API behavior.

## Current Commands

```sh
go run ./cmd/llmrelay version
go run ./cmd/llmrelay init
go run ./cmd/llmrelay config path
go run ./cmd/llmrelay config show
go run ./cmd/llmrelay config validate
go run ./cmd/llmrelay token create local
go run ./cmd/llmrelay token list
go run ./cmd/llmrelay token inspect local
go run ./cmd/llmrelay token verify local --stdin
go run ./cmd/llmrelay upstream set-url https://api.example.test/v1
go run ./cmd/llmrelay upstream set-key --stdin
go run ./cmd/llmrelay upstream test --path /v1/models
go run ./cmd/llmrelay doctor
go run ./cmd/llmrelay upstream show
go run ./cmd/llmrelay serve
go run ./cmd/llmrelay completion bash
```

The `serve` command is still a placeholder for the future HTTP relay stage.

## Development

Run the local checks:

```sh
./scripts/check.sh
```

Or run Go tests directly:

```sh
go test ./...
```

Install the CLI locally:

```sh
make install
```

## Planned Direction

Future work is expected to add request forwarding, streaming responses, access logging, local process management, and usage tracking.

## Security

Do not commit real API keys, relay credentials, deployment URLs, local configuration files, or private planning notes.
