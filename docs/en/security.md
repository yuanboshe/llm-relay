# Security Boundaries

The core boundary of `llm-relay` is: clients only hold relay tokens, while the upstream API key is stored only on the relay host or in a config source controlled by that host.

## Request Flow

```text
client
  Authorization: Bearer llmr_xxx
        ↓
llmrelay
  validate relay token
  resolve key_id
  replace Authorization with upstream API key
        ↓
upstream provider
```

## Places That Should Not Store the Upstream API Key

- remote server
- Caddy or another reverse proxy
- SSH tunnel
- remote client
- LAN client

The remote server and reverse proxy only provide transparent forwarding. They do not perform business authentication.

## Local Files to Protect

- `~/.llmrelay/config.toml`: may contain the upstream API key
- `~/.llmrelay/tokens.json`: contains usable plaintext relay tokens
- `~/.llmrelay/llmrelay.log`: runtime logs should not contain secrets, but should still be treated as sensitive runtime logs

Do not commit these files to Git or sync them to untrusted locations.

## Log Boundaries

The current version does not implement JSONL access logs. Runtime logs are used to observe service startup, listening, tunnel startup, and reconnect events.

Design boundaries:

- Do not record request bodies
- Do not record plaintext Authorization values
- Do not send relay tokens to the upstream provider
- Future access logs may only record necessary metadata, such as time, `key_id`, method, path, status, latency, and bytes_out

## LAN Exposure

When `listen_addr = "0.0.0.0:18080"`, other machines on the LAN can access the relay. Before enabling this, confirm that:

- Relay tokens are created separately per client
- Unused tokens have been disabled, rotated, or deleted
- The local firewall only allows the required network range
- Clients use relay tokens, not the upstream API key

## Remote Entry

The recommended remote entry uses:

- HTTPS / Caddy on the remote server
- SSH reverse tunnel
- remote bind `127.0.0.1:<remote_port>`

The remote server does not store the upstream API key and does not need to store relay tokens.
