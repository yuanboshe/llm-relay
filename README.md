# llm-relay

`llm-relay` is a Go CLI for running a local LLM API relay. Its command-line binary is `llmrelay`.

The current repository contains local configuration, relay token management, single-upstream configuration, HTTP request forwarding, optional SSH reverse tunnel support with reconnects, background process commands, diagnostic commands, and local install support. It does not yet implement usage tracking, quotas, or rate limits.

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
go run ./cmd/llmrelay start
go run ./cmd/llmrelay stop
go run ./cmd/llmrelay restart
go run ./cmd/llmrelay status
go run ./cmd/llmrelay logs
go run ./cmd/llmrelay completion bash
```

## Minimal Flow

Initialize local files:

```sh
llmrelay init
```

Configure one upstream:

```sh
llmrelay upstream set-url https://api.example.test/v1
printf '%s\n' "$UPSTREAM_API_KEY" | llmrelay upstream set-key --stdin
llmrelay upstream test --path /v1/models
```

Create a relay token and keep the plaintext value. The token is only printed once:

```sh
llmrelay token create local --name "Local client"
```

Run the relay in the foreground:

```sh
llmrelay serve
```

Run the relay in the background:

```sh
llmrelay start
llmrelay status
llmrelay logs --tail 50
```

## Configuration

The default configuration file is `~/.llmrelay/config.toml`. It uses TOML syntax:

```toml
listen_addr = "0.0.0.0:18080"

[upstream]
base_url = "https://api.example.test/v1"
api_key_source = "inline"
api_key_env = ""
api_key = ""

[tunnel]
enabled = false
ssh_host = ""
ssh_user = ""
ssh_port = "22"
remote_host = "127.0.0.1"
remote_port = "18080"
```

`tokens.json` is managed by `llmrelay token ...` commands. Relay tokens are stored as SHA-256 hashes, not plaintext. Background runs write `~/.llmrelay/llmrelay.pid` and append process output to `~/.llmrelay/llmrelay.log`.

## LAN Entry

For another machine on the same LAN, bind the relay to a LAN-reachable address:

```toml
listen_addr = "0.0.0.0:18080"
```

The LAN client can then use:

```text
base_url = http://relay-host-lan-ip:18080
api_key = llmr_xxx
```

## Remote Server Entry

`llmrelay` can also ask OpenSSH to create a reverse tunnel to a remote server. Enable the tunnel in `config.toml`:

```toml
[tunnel]
enabled = true
ssh_host = "relay-server"
ssh_user = "ubuntu"
ssh_port = "22"
remote_host = "127.0.0.1"
remote_port = "18080"
```

The generated OpenSSH command is equivalent to:

```sh
ssh -N -T -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3 \
  -p 22 \
  -R 127.0.0.1:18080:127.0.0.1:18080 \
  ubuntu@relay-server
```

On the remote server, a reverse proxy such as Caddy can expose HTTPS:

```caddy
llm.example.test {
    reverse_proxy 127.0.0.1:18080
}
```

The remote client can then use:

```text
base_url = https://llm.example.test
api_key = llmr_xxx
```

When the tunnel exits unexpectedly, `llmrelay serve` retries it with backoff and writes tunnel state changes to the process log.

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

Future work is expected to add access logging, usage tracking, quotas, and rate limits.

## Security

Do not commit real API keys, relay credentials, deployment URLs, local configuration files, or private planning notes.
