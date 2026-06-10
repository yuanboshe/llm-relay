# Token Management

Clients use relay tokens to access `llmrelay`. After `llmrelay` validates a token, it replaces `Authorization: Bearer llmr_xxx` in the request with the upstream API key.

Relay tokens must not be sent to the upstream provider.

## Create

```sh
llmrelay token create remote-client
```

Copy the relay token from the output to the corresponding client:

```text
key-id: remote-client
relay token: llmr_xxx
```

## List and Show

```sh
llmrelay token list
llmrelay token show remote-client
```

`list` and `show` print plaintext relay tokens so they can be copied to clients. Treat command output as sensitive credentials.

## Disable and Enable

```sh
llmrelay token disable remote-client
llmrelay token enable remote-client
```

Disabled tokens are rejected. Unknown tokens return 401, and disabled tokens return 403.

## Delete and Rotate

```sh
llmrelay token rotate remote-client
llmrelay token delete remote-client
```

Rotation generates a new token and invalidates the old token. Deletion removes the record for that `key_id`.

## tokens.json

`tokens.json` is managed by `llmrelay token ...` commands. The current version stores relay tokens in plaintext. Internally, the hash field may remain for compatibility with old records, but the user command surface treats plaintext tokens as authoritative.

Example:

```json
[
  {
    "key_id": "remote-client",
    "token": "llmr_xxx",
    "token_hash": "sha256:<hex>",
    "created_at": "2026-06-10T00:00:00Z",
    "rotated_at": "",
    "enabled": true
  }
]
```

`tokens.json` contains usable relay credentials. Do not sync, leak, or commit this file.
