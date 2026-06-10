# Command Reference

This page is the user command reference. `test` uses positional arguments: the first argument is `key-id`, and the second argument is an optional URL. `upstream` is a `test` subcommand. `key-id` is only a token name, not a subcommand name.

## Install and Uninstall

Install from a local single-file binary:

```sh
chmod +x ./llmrelay-darwin-arm64
./llmrelay-darwin-arm64 install
```

`install` installs the current binary under the unified command name `llmrelay`, creates missing default config and token files, and configures PATH and completion for macOS zsh environments. Re-running it preserves existing config and token files. `--skip-shell-init` and `--skip-completion` are available for scripted installs.

Uninstall the current user install:

```sh
llmrelay uninstall --yes
llmrelay uninstall --dry-run
llmrelay uninstall --yes --purge
llmrelay uninstall --yes --remove-cloudflared
```

By default, `uninstall` removes only the program and shell integration. It does not delete `~/.llmrelay`. `--purge` deletes config, token, and log data. `--remove-cloudflared` attempts to remove the connector service only on macOS. `--dry-run` only previews the operation.

## Main Path

```sh
llmrelay setup
llmrelay start
llmrelay test
llmrelay test upstream
llmrelay test remote-client
llmrelay test remote-client https://llm.example.test
```

`setup` interactively configures the upstream, creates a default relay token, and can optionally configure a Cloudflare remote entry. When run again, it first shows the current configuration state and keeps existing values by default. It overwrites a value only when you choose to update it and enter a new value.

`test` and `test upstream` both check upstream connectivity. `test remote-client` tests the local relay with the relay token named `remote-client`; `test remote-client https://llm.example.test` tests a public entry with the same token. `remote-client` is only an example key-id. Replace it with your own token name.

On success, `test` prints `upstream ok` or `relay ok`, the target URL, key-id, and a partial model list so you can quickly confirm the loop.

## Background Service

```sh
llmrelay start
llmrelay stop
llmrelay restart
llmrelay status
llmrelay logs
llmrelay logs --tail 50
```

`start` is idempotent: it starts the service if it is not running; if it is already running, it only prints `already running` and the current status. It does not implicitly restart the service.

`restart` is the only explicit restart entry. It stops the existing background service and starts it again.

On macOS, the background service uses a user LaunchAgent and starts automatically after login. Other systems use local pid/log process management.

`logs` reads `~/.llmrelay/llmrelay.log` by default. `--tail <n>` controls how many trailing lines are printed.

`status` shows the background service state and, when available, the `ssh-tunnel` enabled state and remote address.

## Configuration

```sh
llmrelay config show
llmrelay config validate
llmrelay config set <key> [value]
```

`config show` displays the current config but does not print the upstream API key in plaintext.

`config validate` only validates local config. It does not make network requests.

`config set` is the only config write entry. Common examples:

```sh
llmrelay config set upstream.base_url https://api.example.test/v1
llmrelay config set upstream.api_key
printf '%s\n' "$UPSTREAM_API_KEY" | llmrelay config set upstream.api_key -
llmrelay config set upstream.api_key_env OPENAI_API_KEY
llmrelay config set listen_addr 127.0.0.1:18080
llmrelay config set tunnel.enabled false
```

`config set upstream.api_key` enters interactive input when no value is passed.

`config set upstream.api_key -` reads the value from stdin, which is useful for scripts.

`config set upstream.api_key_env OPENAI_API_KEY` stores the environment variable name. It does not write the upstream API key to the config file.

`public_url` is no longer a config field. To test a public entry, pass the URL as the second positional argument to `llmrelay test <key-id> <url>`.

`config set` supports dotted TOML paths. Known fields participate in runtime behavior. Unknown fields are preserved in the config file, but `config validate` reports a warning.

## Token Management

```sh
llmrelay token create [key-id]
llmrelay token list
llmrelay token show <key-id>
llmrelay token rotate <key-id>
llmrelay token enable <key-id>
llmrelay token disable <key-id>
llmrelay token delete <key-id>
```

`token create` creates a new relay token. If no `key-id` is passed, it uses the default key ID.

`token list` and `token show` print relay tokens in plaintext so they can be copied to clients. Treat this output as sensitive credentials. Do not commit, screenshot, or sync it to uncontrolled locations.

`token rotate` generates a new token for an existing `key-id`; the old token becomes invalid immediately.

`token disable` and `token enable` temporarily disable or restore a relay token.

## Advanced and Debugging

```sh
llmrelay serve
llmrelay serve --addr 127.0.0.1:18080
llmrelay doctor
llmrelay version
llmrelay version -v
llmrelay completion zsh
llmrelay completion bash
llmrelay completion fish
llmrelay completion powershell
```

`serve` runs the HTTP relay in the foreground and is useful for debugging. Use `start` for daily background operation.

`serve --addr <addr>` temporarily overrides the listen address without writing to the config file.

`doctor` checks the local environment and config without printing sensitive values.

`version` prints only the version by default. `version -v` / `version --verbose` prints commit and build date.

`completion <shell>` generates shell completion scripts. Regular macOS users usually do not need to run this manually because `install` installs zsh completion by default.
