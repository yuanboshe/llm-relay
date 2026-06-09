# llm-relay

`llmrelay` is a local LLM API relay compatible with Anthropic Messages API and OpenAI-compatible APIs.

## Features

- CLI project in Go with binary `llmrelay`
- JSON config in `~/.llmrelay/config.json`
- Relay token auth (`llmr_...`) with **SHA-256 hash-only storage**
- No plaintext relay token persistence
- Upstream API key substitution (client `Authorization` is replaced before forwarding)
- Endpoint support:
  - `/v1/messages`
  - `/v1/models`
  - `/v1/chat/completions`
  - `/v1/responses`
  - `/v1/completions`
  - `/v1/embeddings`
- Streaming/SSE responses are proxied as-is
- JSON-line logs in `~/.llmrelay/relay.log`

## Build

```bash
go build -o llmrelay ./cmd/llmrelay
```

## Commands

```bash
llmrelay init
llmrelay upstream set --provider openai --base-url https://api.openai.com --api-key YOUR_KEY
llmrelay upstream show
llmrelay token create
llmrelay token list
llmrelay token revoke <token-id>
llmrelay serve --addr 127.0.0.1:11434
llmrelay start
llmrelay stop
llmrelay status
llmrelay logs --tail 50
llmrelay doctor
```

## Security Notes

- Relay tokens are generated with prefix `llmr_`.
- Only SHA-256 token hashes are stored in config.
- Upstream API keys are masked in `upstream show`.
- Request logs do not include upstream API keys.
