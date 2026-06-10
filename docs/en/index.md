# llm-relay

`llm-relay` is an LLM API relay tool that runs locally or on a server. After it validates a relay token from a client, it replaces that token with the upstream provider API key and forwards the request to the configured upstream `base_url`.

It is useful when one controlled machine can access an upstream LLM service and you want to expose that capability safely to remote clients or other clients on the same LAN.

## Current Capabilities

- The installed command name is always `llmrelay`.
- Supports interactive `setup` for one upstream and a relay token.
- Supports `config show`, `config validate`, and `config set <key> [value]`.
- Supports relay token create, list, show, enable, disable, delete, and rotate.
- Supports a real HTTP relay: token authentication, allowed paths, Authorization replacement, and streaming / SSE forwarding.
- Supports LAN access by exposing the relay through `listen_addr`.
- Supports remote access: Cloudflare Tunnel is recommended, with SSH reverse tunnel kept as an advanced option.
- Supports background service commands: `start`, `stop`, `restart`, `status`, and `logs`.

## Current Non-Goals

- Does not promise full protocol coverage for every OpenAI-compatible API or Anthropic Messages API behavior.
- Does not yet implement JSONL access logs.
- Does not yet implement usage tracking, cached token statistics, quotas, or rate limits.
- Does not yet implement a dashboard, multi-provider management, a complex permission model, or macOS Keychain.

## Recommended Reading Order

1. [Quick Start](./quickstart.md)
2. [Build from Source](./build.md)
3. [Command Reference](./commands.md)
4. [Deployment Loop](./deploy.md)
5. [Configuration](./config.md)
6. [Token Management](./tokens.md)
7. [Security Boundaries](./security.md)
8. [Troubleshooting](./troubleshooting.md)
9. [Local Documentation Site](./docs-site.md)
