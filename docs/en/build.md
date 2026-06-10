# Build from Source

This page explains how to build single-file `llmrelay` release binaries from source. Run all commands from the public repository root.

## Default Build

```sh
make build
```

`make build` builds the default release targets and writes artifacts to `dist/`:

```text
dist/llmrelay-linux-amd64
dist/llmrelay-linux-arm64
dist/llmrelay-windows-amd64.exe
dist/llmrelay-darwin-amd64
dist/llmrelay-darwin-arm64
dist/SHA256SUMS
```

## Build Targets

Common targets:

```sh
make build-local
make build-linux-amd64
make build-linux-arm64
make build-windows-amd64
make build-darwin-amd64
make build-darwin-arm64
make clean
```

`make build-local` builds only the current machine's `GOOS/GOARCH`. Platform targets build one binary for that platform. `make clean` removes `dist/` and `coverage.out`.

## Version Parameters

Build metadata is passed through Make variables:

```sh
make build VERSION=v0.1.0
make build VERSION=v0.1.0 COMMIT=abc1234 BUILD_DATE=2026-06-11T00:00:00Z
```

Rules:

- `VERSION` defaults to `v0.0.0`. Official release builds should set it explicitly, for example `VERSION=v0.1.0`.
- `COMMIT` defaults to the current short Git commit. Override it only when the source is not from a Git checkout or you need a fixed displayed value.
- When `BUILD_DATE` is empty, the build script writes the current UTC time. If you set it manually, use RFC3339 format, for example `2026-06-11T00:00:00Z`.

## Build and Install

`make build` only builds binaries. It does not install anything to the user directory or write config files. The project does not provide `make install` as the official installation path.

To install a locally built binary, use the install script with `--local`:

```sh
sh ./scripts/install.sh --local ./dist/llmrelay-darwin-arm64
```
