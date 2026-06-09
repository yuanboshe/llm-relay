# llm-relay

`llm-relay` is a Go CLI for running a local LLM API relay. Its command-line binary is `llmrelay`.

The current repository contains user-level self-installation, local configuration, relay token management, single-upstream configuration, HTTP request forwarding, optional SSH reverse tunnel support with reconnects, background process commands, diagnostic commands, and local install support. It does not yet implement usage tracking, quotas, or rate limits.

## Current Commands

```sh
go run ./cmd/llmrelay install
go run ./cmd/llmrelay setup
go run ./cmd/llmrelay version
go run ./cmd/llmrelay config show
go run ./cmd/llmrelay config validate
go run ./cmd/llmrelay config set-url https://api.example.test/v1
go run ./cmd/llmrelay config set-key --stdin
go run ./cmd/llmrelay config test --path /v1/models
go run ./cmd/llmrelay token create local
go run ./cmd/llmrelay token list
go run ./cmd/llmrelay token inspect local
go run ./cmd/llmrelay token verify local --stdin
go run ./cmd/llmrelay doctor
go run ./cmd/llmrelay serve
go run ./cmd/llmrelay start
go run ./cmd/llmrelay stop
go run ./cmd/llmrelay restart
go run ./cmd/llmrelay status
go run ./cmd/llmrelay logs
go run ./cmd/llmrelay completion bash
```

## Minimal Flow

Install a downloaded macOS binary. The downloaded file may include a platform suffix; the installed command is always `llmrelay`:

```sh
chmod +x ./llmrelay-darwin-arm64
./llmrelay-darwin-arm64 install
```

The installer copies the binary to `~/Library/Application Support/llmrelay/bin/llmrelay`, creates a `~/.local/bin/llmrelay` command link, initializes `~/.llmrelay/config.toml` and `~/.llmrelay/tokens.json` if missing, updates `~/.zshrc` for PATH and zsh completion, and preserves existing config and token files.

Configure one upstream:

```sh
llmrelay setup
```

Or configure it with scriptable commands:

```sh
llmrelay config set-url https://api.example.test/v1
printf '%s\n' "$UPSTREAM_API_KEY" | llmrelay config set-key --stdin
llmrelay config test --path /v1/models
```

Create additional relay tokens as needed and keep the plaintext values. A token is only printed once:

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

On macOS, `llmrelay start` uses a user LaunchAgent so the relay starts again when you log in. `llmrelay stop` unloads that LaunchAgent.

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

`tokens.json` is managed by `llmrelay token ...` commands. Relay tokens are stored as SHA-256 hashes, not plaintext. Process output is appended to `~/.llmrelay/llmrelay.log`; non-macOS background runs also write `~/.llmrelay/llmrelay.pid`.

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

## Planned Direction

Future work is expected to add access logging, usage tracking, quotas, and rate limits.

## Security

Do not commit real API keys, relay credentials, deployment URLs, local configuration files, or private planning notes.
