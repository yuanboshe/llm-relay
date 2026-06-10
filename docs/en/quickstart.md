# Quick Start

This page shows the minimal flow from installation to starting the relay. The command examples use the installed `llmrelay` command, not `go run ./cmd/llmrelay ...` from the source tree.

## 1. Install

The recommended path is to install the latest release with the install script. On macOS, the script clears downloaded-file attributes and applies a local ad-hoc signature before the first binary run, preventing the system from killing an untreated downloaded binary.

```sh
curl -fsSL https://raw.githubusercontent.com/yuanboshe/llm-relay/main/scripts/install.sh | sh
```

If you already downloaded a local binary, install it with the same script:

```sh
curl -fsSLO https://raw.githubusercontent.com/yuanboshe/llm-relay/main/scripts/install.sh
sh ./install.sh --local ./llmrelay-darwin-arm64
```

The installer:

- Copies the binary to `~/Library/Application Support/llmrelay/bin/llmrelay`
- Creates the `~/.local/bin/llmrelay` command link
- Initializes missing `~/.llmrelay/config.toml` and `~/.llmrelay/tokens.json`
- Configures PATH and completion for zsh
- Preserves existing config and token files

## 2. First Configuration

Run the interactive setup wizard to configure one upstream and create a default relay token.

```sh
llmrelay setup
```

You can also configure the upstream with scriptable commands:

```sh
llmrelay config set upstream.base_url https://api.example.test/v1
llmrelay config set upstream.api_key
llmrelay test upstream
```

If the upstream models endpoint returns model IDs, `llmrelay test upstream` prints a few model names so you can confirm the test reached a usable upstream.

## 3. Create Extra Tokens

Create relay tokens per client or use case.

```sh
llmrelay token create remote-client
llmrelay token create lan-client
llmrelay token show remote-client
```

`tokens.json` stores usable relay tokens in plaintext. This file should be readable and writable only by the local user. Do not sync, leak, or commit it.

## 4. Run in the Foreground

For troubleshooting or temporary use, run the relay in the foreground:

```sh
llmrelay serve
```

## 5. Run in the Background

For long-running use, start it as a background service:

```sh
llmrelay doctor
llmrelay start
llmrelay test remote-client
llmrelay status
llmrelay logs --tail 50
```

On macOS, `llmrelay start` uses a user LaunchAgent and starts automatically after login. On non-macOS systems, background mode uses `~/.llmrelay/llmrelay.pid` and `~/.llmrelay/llmrelay.log`.

## 6. Client Usage

Clients use the relay address as `base_url` and the relay token as `api_key`.

```text
base_url = http://relay-host-lan-ip:18080
api_key = llmr_xxx
```

Prefer the built-in test command when validating:

```sh
llmrelay test remote-client
```

When testing a remote entry, use `test <key-id> <url>`. The command can read `public_url` from config, or you can pass a temporary URL:

```sh
llmrelay test remote-client https://relay.example.test
```
