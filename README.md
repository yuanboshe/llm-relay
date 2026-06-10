# llm-relay

`llm-relay` is a Go CLI for running a local LLM API relay. Its command-line binary is `llmrelay`.

Current release line: `v0.1.x`. Source builds default to `v0.0.0`; if a binary reports `v0.0.0`, it was built without an explicit release version.

The repository provides user-level installation and uninstallation, local configuration, relay token management, HTTP request forwarding, optional SSH reverse tunnel support with reconnects, background process commands, and diagnostics. It does not yet implement usage tracking, quotas, or rate limits.

## Build from Source

To build release binaries from source, run `make build` from the repository root:

```sh
make build
```

This builds the default release targets and writes them to `dist/` with matching `SHA256SUMS` checksums.

Common targets:

```sh
make build
make build-local
make build-linux-amd64
make build-linux-arm64
make build-windows-amd64
make build-darwin-amd64
make build-darwin-arm64
make clean
```

Build metadata is passed as Make variables:

```sh
make build VERSION=v0.1.0
make build VERSION=v0.1.0 COMMIT=abc1234 BUILD_DATE=2026-06-11T00:00:00Z
```

`VERSION` defaults to `v0.0.0`, `COMMIT` defaults to the current short Git commit, and an empty `BUILD_DATE` is replaced with the current UTC time. Use an explicit `VERSION` for release builds.

## Documentation

User documentation is available at <https://yuanboshe.github.io/llm-relay/>.
For the full command reference, see [docs/en/commands.md](./docs/en/commands.md).
Chinese documentation is also available at [docs/zh/](./docs/zh/).

## Quick Start

Install the latest release with the install script. On macOS, the script clears downloaded-file attributes and applies a local ad-hoc signature before the first binary run:

```sh
curl -fsSL https://raw.githubusercontent.com/yuanboshe/llm-relay/main/scripts/install.sh | sh
```

To install from a local binary, pass it to the same script:

```sh
curl -fsSLO https://raw.githubusercontent.com/yuanboshe/llm-relay/main/scripts/install.sh
sh ./install.sh --local ./llmrelay-darwin-arm64
```

The installer copies the binary to `~/Library/Application Support/llmrelay/bin/llmrelay`, creates a `~/.local/bin/llmrelay` command link, initializes `~/.llmrelay/config.toml` and `~/.llmrelay/tokens.json` if missing, updates `~/.zshrc` for PATH and zsh completion, and preserves existing config and token files.

Run the setup wizard to configure one upstream and create a relay token:

```sh
llmrelay setup
```

Then start the background service and test the token you created during setup:

```sh
llmrelay start
llmrelay test <key-id>
```

Replace `<key-id>` with the relay token name you created in `setup`.

Use `llmrelay uninstall --yes` to remove the user install later. Add `--purge` to remove `~/.llmrelay`, or `--remove-cloudflared` on macOS to remove the connector service too.
