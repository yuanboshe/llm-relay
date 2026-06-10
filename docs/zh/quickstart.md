# 快速开始

本页展示从安装到启动 relay 的最小流程。命令示例默认使用安装后的 `llmrelay`，不是源码目录里的 `go run ./cmd/llmrelay ...`。

## 1. 安装

下载的二进制文件可以带平台后缀；安装后命令名固定为 `llmrelay`。

```sh
chmod +x ./llmrelay-darwin-arm64
./llmrelay-darwin-arm64 install
```

安装器会：

- 复制二进制到 `~/Library/Application Support/llmrelay/bin/llmrelay`
- 创建 `~/.local/bin/llmrelay` 命令链接
- 初始化缺失的 `~/.llmrelay/config.toml` 和 `~/.llmrelay/tokens.json`
- 为 zsh 配置 PATH 和 completion
- 保留已有配置文件和 token 文件

## 2. 首次配置

运行交互式 setup，配置一个 upstream，并创建一个默认 relay token。

```sh
llmrelay setup
```

也可以用脚本化命令配置 upstream：

```sh
llmrelay config set upstream.base_url https://api.example.test/v1
llmrelay config set upstream.api_key
llmrelay test upstream
```

## 3. 创建额外 token

按客户端或使用场景创建 relay token。

```sh
llmrelay token create remote-client
llmrelay token create lan-client
llmrelay token show remote-client
```

`tokens.json` 会保存可直接使用的 relay token 明文。这个文件应仅限本机用户可读写，不要同步、泄漏或提交。

## 4. 前台运行

排查或临时使用时，可以前台运行：

```sh
llmrelay serve
```

## 5. 后台运行

长期使用时，建议后台运行：

```sh
llmrelay doctor
llmrelay start
llmrelay test local
llmrelay status
llmrelay logs --tail 50
```

macOS 上，`llmrelay start` 使用用户 LaunchAgent，登录后会自动启动。非 macOS 后台模式使用 `~/.llmrelay/llmrelay.pid` 和 `~/.llmrelay/llmrelay.log`。

## 6. 客户端使用

客户端使用 relay 地址作为 `base_url`，使用 relay token 作为 `api_key`。

```text
base_url = http://relay-host-lan-ip:18080
api_key = llmr_xxx
```

验证时优先使用内置命令：

```sh
llmrelay test local
```
