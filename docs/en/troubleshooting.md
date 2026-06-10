# Troubleshooting

Start from four areas: local config, upstream connectivity, background service state, and tunnel state.

## Config Path

Show config:

```sh
llmrelay config show
```

Validate config:

```sh
llmrelay config validate
```

If you use a custom directory, confirm that the environment variable is consistent:

```sh
echo "$LLMRELAY_HOME"
```

## Upstream Connectivity

Test the upstream from the relay host:

```sh
llmrelay test upstream
```

If it fails, check:

- Whether `upstream.base_url` is correct
- Whether the API key source is correct
- Whether the environment variable exists when `api_key_source = "env"`
- Whether the upstream provider allows access from the current relay host

## Local Service

Show status and logs:

```sh
llmrelay status
llmrelay logs --tail 100
llmrelay test <key-id>
```

For troubleshooting, run in the foreground:

```sh
llmrelay serve
```

Common cases:

- Missing Authorization: returns 401
- Non-`llmr_` bearer token: returns 401
- Unknown token: returns 401
- Disabled token: returns 403
- Non-allowed path: returns 404 or 403

## tunnel

If the remote entry is unavailable, first confirm that system OpenSSH works:

```sh
ssh relay-server
```

Then check whether Caddy on the remote server can access the tunnel bind port:

```sh
curl http://127.0.0.1:18080/v1/models
```

If Caddy returns 502:

- Check `llmrelay status`
- Check `llmrelay logs --tail 100`
- Confirm that `remote_port` matches the Caddy reverse proxy port
- Confirm that sshd on the remote server allows reverse tunnels

## LAN Entry

When a LAN client cannot connect, check:

- Whether `listen_addr` is `0.0.0.0:<port>` or the relay host LAN IP
- Whether the local firewall allows that port
- Whether the LAN client's `base_url` points to the relay host
- Whether the LAN client uses the relay token, not the upstream API key

## Capabilities Not Yet Implemented

The following capabilities are not current troubleshooting entry points:

- JSONL access log
- `usage.sqlite`
- cached token statistics
- Per-relay-token quotas or rate limits
