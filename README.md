# llm-relay

`llm-relay` is a Go CLI for running a local LLM API relay. Its command-line binary is `llmrelay`.

Current release line: `v0.1.x`. Source builds default to `v0.0.0`; if a binary reports `v0.0.0`, it was built without an explicit release version.

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
llmrelay config set upstream.base_url https://api.example.test/v1
llmrelay config set upstream.api_key
llmrelay config set upstream.api_key -
llmrelay config set upstream.api_key_env OPENAI_API_KEY
llmrelay config set listen_addr 127.0.0.1:18080
llmrelay config set public_url https://llm.example.test
llmrelay token create local
llmrelay token list
llmrelay token show local
llmrelay doctor
llmrelay serve
llmrelay test
llmrelay test upstream
llmrelay test local
llmrelay test public
llmrelay test public https://llm.example.test
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

Run the setup wizard to configure one upstream and create a relay token. The wizard is safe to run again: existing values are shown first and are kept unless you choose to update them.

```sh
llmrelay setup
```

Or configure it with scriptable commands:

```sh
llmrelay config set upstream.base_url https://api.example.test/v1
printf '%s\n' "$UPSTREAM_API_KEY" | llmrelay config set upstream.api_key -
llmrelay test upstream
```

Create additional relay tokens as needed. `tokens.json` stores relay tokens in plaintext, so keep that file private:

```sh
llmrelay token create local
llmrelay token show local
```

Run the relay in the foreground:

```sh
llmrelay serve
```

Run the relay in the background and test it without writing curl commands by hand:

```sh
llmrelay start
llmrelay status
llmrelay logs --tail 50
llmrelay test
```

On macOS, `llmrelay start` uses a user LaunchAgent so the relay starts again when you log in. `llmrelay stop` unloads that LaunchAgent.
If `llmrelay start` is run while the service is already running, it reports `already running` and does not restart the process. Use `llmrelay restart` when you explicitly want to stop and start the service.

## Configuration

The default configuration file is `~/.llmrelay/config.toml`. It uses TOML syntax:

```toml
listen_addr = "0.0.0.0:18080"
public_url = "https://llm.example.test"

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

`tokens.json` is managed by `llmrelay token ...` commands. Relay tokens are stored in plaintext in that local file and are printed by `llmrelay token list` and `llmrelay token show <key-id>`. Keep this file and command output private. Process output is appended to `~/.llmrelay/llmrelay.log`; non-macOS background runs also write `~/.llmrelay/llmrelay.pid`.

Example token store:

```json
[
  {
    "key_id": "local",
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

## Cloudflare Tunnel Entry

For remote access without a server or SSH, create a Cloudflare Tunnel and route a public hostname to the relay host:

```text
Public hostname: llm.example.test
Service type: HTTP
Service URL: http://127.0.0.1:18080
```

The Cloudflare administrator gives the relay host user:

```text
TUNNEL_TOKEN=<cloudflare-tunnel-token>
PUBLIC_URL=https://llm.example.test
```

During `llmrelay setup`, choose the default Cloudflare remote access flow and paste the tunnel token. On macOS, setup installs and starts the `cloudflared` service if it is not already installed:

```sh
brew install cloudflared
sudo cloudflared service install <TUNNEL_TOKEN>
sudo launchctl start com.cloudflare.cloudflared
```

The token is not stored in `config.toml`. Cloudflare Tunnel does not use the `[tunnel]` SSH settings, so keep them disabled:

```toml
[tunnel]
enabled = false
```

If `cloudflared` is already installed and loaded, `setup` skips reinstalling it by default. Choose update only when you want to replace the connector token.

Save the public URL and test the public entry from the relay host:

```sh
llmrelay config set public_url https://llm.example.test
llmrelay test public
```

OpenAI-compatible clients usually use:

```text
base_url = https://llm.example.test/v1
api_key = llmr_xxx
```

Anthropic-compatible clients usually use:

```text
base_url = https://llm.example.test
api_key = llmr_xxx
```

## Advanced SSH Remote Entry

`llmrelay` can also ask OpenSSH to create a reverse tunnel to a remote server. Enable the tunnel in `config.toml`:

Before enabling the tunnel, make sure OpenSSH can log in without an interactive password prompt. This must succeed from the relay host:

```sh
ssh -o BatchMode=yes ubuntu@relay-server true
```

If the relay host needs a dedicated key, put that key in `~/.ssh/config` and use the configured host alias as `ssh_host`:

```sshconfig
Host relay-server
    HostName ssh.example.test
    User ubuntu
    Port 22
    IdentityFile ~/.ssh/id_ed25519
    IdentitiesOnly yes
```

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

OpenAI-compatible remote clients can then use:

```text
base_url = https://llm.example.test/v1
api_key = llmr_xxx
```

When the tunnel exits unexpectedly, `llmrelay serve` retries it with backoff and writes tunnel state changes to the process log.

## Development

Run the CLI from source:

```sh
go run ./cmd/llmrelay version
```

`llmrelay version` prints only the version for normal use. Build metadata is available when troubleshooting:

```sh
llmrelay version -v
llmrelay version --verbose
```

Build cross-platform single-file binaries. The default source-build version is `v0.0.0`; pass `VERSION=...` for release builds. The build strips Go symbols with linker flags and does not use UPX. Build date defaults to the current UTC time:

```sh
make build
```

Build a versioned release:

```sh
make build VERSION=v0.1.0
```

Release outputs:

```text
dist/llmrelay-linux-amd64
dist/llmrelay-linux-arm64
dist/llmrelay-windows-amd64.exe
dist/llmrelay-darwin-amd64
dist/llmrelay-darwin-arm64
dist/SHA256SUMS
```

Each file is directly executable after permissions are set. For example, download `llmrelay-darwin-arm64`, run `chmod +x ./llmrelay-darwin-arm64`, then `./llmrelay-darwin-arm64 install`; install copies it to the final `llmrelay` command. Build assets are written to `dist/`, the repository's existing ignored distribution-output directory.

Build a single target when needed:

```sh
make build-darwin-arm64
make build-windows-amd64
make build-local
```

Run the local checks:

```sh
./scripts/check.sh
```

Or run Go tests directly:

```sh
go test ./...
```

Run the documentation site locally:

```sh
npm ci
npm run docs:dev
```

Build and preview the static documentation site:

```sh
npm run docs:build
npm run docs:preview
```

The local documentation URL uses the GitHub Pages project base path:

```text
http://localhost:5173/llm-relay/
```

## Planned Direction

Future work is expected to add access logging, usage tracking, quotas, and rate limits.

## Security

Do not commit real API keys, relay credentials, deployment URLs, local configuration files, or private planning notes.
