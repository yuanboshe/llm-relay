# llm-relay

`llm-relay` is a Go CLI for running a local LLM API relay. Its command-line binary is `llmrelay`.

The current repository contains user-level self-installation, local configuration, relay token management, single-upstream configuration, HTTP request forwarding, optional SSH reverse tunnel support with reconnects, background process commands, and diagnostic commands. It does not yet implement usage tracking, quotas, or rate limits.

## Documentation

User documentation is available at <https://yuanboshe.github.io/llm-relay/>.

## Current Commands

```sh
llmrelay install
llmrelay setup
llmrelay version
llmrelay config show
llmrelay config validate
llmrelay config set-url https://api.example.test/v1
llmrelay config set-key --stdin
llmrelay config test --path /v1/models
llmrelay token create local
llmrelay token list
llmrelay token inspect local
llmrelay token verify local --stdin
llmrelay doctor
llmrelay serve
llmrelay start
llmrelay stop
llmrelay restart
llmrelay status
llmrelay logs
llmrelay completion bash
```

## Minimal Flow

Install a downloaded macOS binary. The downloaded file may include a platform suffix; the installed command is always `llmrelay`:

```sh
chmod +x ./llmrelay-darwin-arm64
./llmrelay-darwin-arm64 install
```

The installer copies the binary to `~/Library/Application Support/llmrelay/bin/llmrelay`, creates a `~/.local/bin/llmrelay` command link, initializes `~/.llmrelay/config.toml` and `~/.llmrelay/tokens.json` if missing, updates `~/.zshrc` for PATH and zsh completion, and preserves existing config and token files.

Run the first-time setup wizard to configure one upstream and create a relay token:

```sh
llmrelay setup
```

Or configure it with scriptable commands:

```sh
llmrelay config set-url https://api.example.test/v1
printf '%s\n' "$UPSTREAM_API_KEY" | llmrelay config set-key --stdin
llmrelay config test --path /v1/models
```

Create additional relay tokens as needed. `tokens.json` stores relay tokens in plaintext, so keep that file private:

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

`tokens.json` is managed by `llmrelay token ...` commands. Relay tokens are stored in plaintext in that local file, with SHA-256 hashes kept for compatibility and verification. Keep this file private. Process output is appended to `~/.llmrelay/llmrelay.log`; non-macOS background runs also write `~/.llmrelay/llmrelay.pid`.

Example token store:

```json
[
  {
    "key_id": "local",
    "name": "Local client",
    "note": "",
    "token": "llmr_xxx",
    "token_hash": "sha256:<hex>",
    "created_at": "2026-06-10T00:00:00Z",
    "rotated_at": "",
    "enabled": true
  }
]
```

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

```text
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

Run the CLI from source:

```sh
go run ./cmd/llmrelay version
```

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
