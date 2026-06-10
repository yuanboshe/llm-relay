# Deployment Loop

The deployment goal for `llm-relay` is to keep the upstream API key on the controlled relay host. Cloudflare, the remote server, reverse proxy, SSH tunnel, and clients only handle relay tokens. They do not store the upstream API key.

## LAN Entry

A LAN entry lets clients on the same LAN access the relay host.

```text
LAN client
  base_url = http://relay-host-lan-ip:18080
  api_key = llmr_xxx
        ↓
relay host llmrelay
  validate relay token
  replace Authorization
        ↓
upstream LLM provider
```

The relay host `config.toml` can listen on all interfaces:

```toml
listen_addr = "0.0.0.0:18080"
```

LAN clients use:

```text
base_url = http://relay-host-lan-ip:18080
api_key = llmr_xxx
```

The first LAN entry uses HTTP + relay token. If you need LAN HTTPS, plan it separately with local Caddy or built-in TLS later.

## Cloudflare Tunnel Entry

Cloudflare Tunnel is the recommended remote entry for users who do not have their own server or do not want to configure SSH.

```text
remote client
  base_url = https://llm.example.test
  api_key = llmr_xxx
        ↓
Cloudflare Public Hostname
        ↓
cloudflared on relay host
        ↓
relay host llmrelay
  validate relay token
  replace Authorization
        ↓
upstream LLM provider
```

The Cloudflare Public Hostname Service URL points to the local relay:

```text
http://127.0.0.1:18080
```

Run on the relay host:

```sh
llmrelay setup
llmrelay start
llmrelay test <key-id>
llmrelay test <key-id> https://llm.example.test
```

Cloudflare Tunnel does not use the `[tunnel]` SSH config:

```toml
[tunnel]
enabled = false
```

## Advanced Entry: Remote Server and SSH Reverse Tunnel

The remote entry lets remote clients access the relay host over HTTPS. The remote server only provides HTTPS and transparent forwarding. It does not store the upstream API key and does not perform relay token authentication.

```text
remote client
  base_url = https://relay.example.test
  api_key = llmr_xxx
        ↓
Caddy on remote server
        ↓
SSH reverse tunnel
        ↓
relay host llmrelay
  validate relay token
  replace Authorization
        ↓
upstream LLM provider
```

Enable tunnel in the relay host `config.toml`:

```toml
[tunnel]
enabled = true
ssh_host = "relay-server"
ssh_user = "ubuntu"
ssh_port = "22"
remote_host = "127.0.0.1"
remote_port = "18080"
```

`llmrelay` uses system OpenSSH to create the reverse tunnel, equivalent to:

```sh
ssh -N -T -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3 \
  -p 22 \
  -R 127.0.0.1:18080:127.0.0.1:18080 \
  ubuntu@relay-server
```

On the remote server, Caddy can expose HTTPS:

```text
relay.example.test {
    reverse_proxy 127.0.0.1:18080
}
```

Remote clients use:

```text
base_url = https://relay.example.test
api_key = llmr_xxx
```

## Verification Checklist

- `llmrelay doctor` does not print the upstream API key or plaintext relay tokens.
- `llmrelay test upstream` succeeds on the relay host; if the upstream returns model IDs, the output shows part of the model list.
- After `llmrelay start`, `llmrelay status` shows the background service running.
- `llmrelay test <key-id>` succeeds on the relay host and shows part of the model list fetched through the relay token.
- `llmrelay test <key-id> <url>` succeeds against the public entry.
- `llmrelay logs --tail 50` shows local listen and tunnel startup information.
- The remote server only connects to `127.0.0.1:<remote_port>` and does not store the upstream API key.
- The client can call `/v1/models` with the relay token.
- The upstream provider never receives the `llmr_` relay token; it only receives the real upstream API key.
