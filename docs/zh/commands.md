# 命令手册

本页按使用场景整理当前 `llmrelay` 命令面。完整命令以当前版本的 `llmrelay help` 和本手册为准。

## 主路径

普通用户推荐流程：

```sh
llmrelay setup
llmrelay start
llmrelay test
```

从下载的单文件二进制自安装：

```sh
chmod +x ./llmrelay-darwin-arm64
./llmrelay-darwin-arm64 install
```

`install` 会把当前二进制安装成统一命令名 `llmrelay`，创建缺失的默认配置和 token 文件，并在 macOS zsh 环境中配置 PATH 和补全。重复执行会保留已有配置和 token 文件。

`setup` 用于交互式配置 upstream、创建默认 relay token，并可选择配置 Cloudflare 远程入口。重复执行时会先展示现有配置状态，默认保留已有值；只有选择更新并输入新值时才覆盖。

## 后台服务

```sh
llmrelay start
llmrelay stop
llmrelay restart
llmrelay status
llmrelay logs
llmrelay logs --tail 50
```

`start` 是幂等启动入口：如果服务没有运行就启动；如果已经运行，只输出 `already running` 和当前状态，不会隐式重启。

`restart` 是唯一明确重启入口，会停止现有后台服务再启动。

macOS 上后台服务使用用户 LaunchAgent，登录后自动启动。其他系统使用本地 pid/log 方式管理后台进程。

`logs` 默认读取 `~/.llmrelay/llmrelay.log`。`--tail <n>` 控制打印末尾行数。

## 测试验收

```sh
llmrelay test
llmrelay test upstream
llmrelay test local
llmrelay test public
llmrelay test public https://llm.example.test
```

`test` 默认执行配置、upstream、本机 relay 检查；如果配置了 `public_url`，也会检查公网入口。

`test upstream` 只验证 upstream `base_url` 和 API key。程序会自动尝试常见 models 路径，不要求用户手动拼 `/models` 或 `/v1/models`。

`test local` 使用本地 token store 中可用的 relay token 测试本机 relay。

`test public` 使用配置里的 `public_url` 测试公网入口。`test public <url>` 临时测试指定公网入口，不写入配置。

成功时输出会包含明确的 ok 状态，并给出 OpenAI-compatible 和 Anthropic-compatible 客户端的 `base_url` 建议。

## 配置

```sh
llmrelay config show
llmrelay config validate
llmrelay config set <key> [value]
```

`config show` 展示当前配置，但不会打印 upstream API key 明文。

`config validate` 只做本地配置校验，不发起网络请求。

`config set` 是唯一配置写入口。常用示例：

```sh
llmrelay config set upstream.base_url https://api.example.test/v1
llmrelay config set upstream.api_key
printf '%s\n' "$UPSTREAM_API_KEY" | llmrelay config set upstream.api_key -
llmrelay config set upstream.api_key_env OPENAI_API_KEY
llmrelay config set listen_addr 127.0.0.1:18080
llmrelay config set listen_addr 0.0.0.0:18080
llmrelay config set public_url https://llm.example.test
llmrelay config set tunnel.enabled false
```

`config set upstream.api_key` 不传 value 时进入交互输入。

`config set upstream.api_key -` 从 stdin 读取值，适合脚本使用。

`config set upstream.api_key_env OPENAI_API_KEY` 保存环境变量名，不把 upstream API key 写入配置文件。

`config set` 支持 dotted TOML path。已知字段会参与运行；未知字段会保留在配置文件中，但 `config validate` 会给出 warning。

## Token 管理

```sh
llmrelay token create [key-id]
llmrelay token list
llmrelay token show <key-id>
llmrelay token rotate <key-id>
llmrelay token enable <key-id>
llmrelay token disable <key-id>
llmrelay token delete <key-id>
```

`token create` 创建新的 relay token。不给 `key-id` 时使用默认 key ID。

`token list` 和 `token show` 会输出 relay token 明文，方便复制到客户端。请把输出当作敏感凭据处理，不要提交、截图或同步到不受控位置。

`token rotate` 会为已有 `key-id` 生成新 token；旧 token 随即失效。

`token disable` 和 `token enable` 用于临时禁用或恢复某个 relay token。

## 高级与调试

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

`serve` 前台运行 HTTP relay，适合调试。日常后台运行使用 `start`。

`serve --addr <addr>` 临时覆盖监听地址，不写入配置文件。

`doctor` 检查本地环境和配置，不打印敏感值。

`version` 默认只输出版本号；`version -v` / `version --verbose` 输出 commit 和 build date。

`completion <shell>` 生成 shell completion 脚本。普通 macOS 用户通过 `install` 默认安装 zsh completion，通常不需要手动执行。

## 已删除旧入口

以下旧入口不是当前命令面的一部分，不应在新文档或脚本中使用：

```text
llmrelay config set-url
llmrelay config set-key
llmrelay config test
llmrelay token inspect
llmrelay token verify
llmrelay test --url
```

公网入口测试使用：

```sh
llmrelay test public https://llm.example.test
```
