# Configuration

The default config directory is `~/.llmrelay`. Override it with `LLMRELAY_HOME=/custom/path`.

## Default Files

- Config file: `~/.llmrelay/config.toml`
- Token store: `~/.llmrelay/tokens.json`
- Background pid file: `~/.llmrelay/llmrelay.pid`
- Background log file: `~/.llmrelay/llmrelay.log`

macOS user-level installs also use:

- Install target: `~/Library/Application Support/llmrelay/bin/llmrelay`
- Command link: `~/.local/bin/llmrelay`
- LaunchAgent: `~/Library/LaunchAgents/com.yuanboshe.llmrelay.plist`

## config.toml

```toml
listen_addr = "127.0.0.1:18080"

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

## listen_addr

`listen_addr` controls the address used by `llmrelay serve`.

- Local access only: `127.0.0.1:18080`
- LAN access: `0.0.0.0:18080`
- Bind only one LAN IP: `192.168.1.10:18080`

When exposing the relay to a LAN, confirm that relay tokens are random enough and check the local firewall.

## upstream

`upstream.base_url` is the upstream provider base URL. `llmrelay` avoids joining the base URL and request path into `/v1/v1/...`.

Supported API key sources:

- `inline`: stored in local `config.toml`
- `env`: read from an environment variable

Scripted configuration example:

```sh
llmrelay config set upstream.base_url https://api.example.test/v1
llmrelay config set upstream.api_key
llmrelay test upstream
```

When using an environment variable, make sure the background service startup environment can read it too.

## tunnel

`[tunnel]` is for the remote server entry. When enabled, `llmrelay serve` starts a system OpenSSH `ssh -R` process and automatically reconnects after disconnection.

```toml
[tunnel]
enabled = true
ssh_host = "relay-server"
ssh_user = "ubuntu"
ssh_port = "22"
remote_host = "127.0.0.1"
remote_port = "18080"
```

SSH config, private keys, ssh-agent, known_hosts, and ProxyJump all reuse system OpenSSH behavior.

See [Command Reference](./commands.md) for related commands.
