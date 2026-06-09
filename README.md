# llm-relay

`llm-relay` is an early Go skeleton for a local or server-side LLM API relay. Its command-line binary is `llmrelay`.

The current repository only contains the initial command-line entry point and internal package boundaries. It does not yet implement a production relay, token authentication, upstream forwarding, usage tracking, or full OpenAI-compatible / Anthropic-compatible API behavior.

## Current Commands

```sh
go run ./cmd/llmrelay version
go run ./cmd/llmrelay config path
go run ./cmd/llmrelay serve
go run ./cmd/llmrelay completion bash
```

The `serve` command currently prints the planned listener and route surface instead of starting a real proxy.

## Development

Run the local checks:

```sh
./scripts/check.sh
```

Or run Go tests directly:

```sh
go test ./...
```

## Planned Direction

Future work is expected to add configuration initialization, upstream provider settings, local relay token management, request forwarding, streaming responses, and access logging.

## Security

Do not commit real API keys, relay credentials, deployment URLs, local configuration files, or private planning notes.
