# 配置

默认配置目录是 `~/.llmrelay`。可以通过 `LLMRELAY_HOME=/custom/path` 覆盖。

## 默认文件

- 配置文件：`~/.llmrelay/config.toml`
- token 存储：`~/.llmrelay/tokens.json`
- 后台 pid 文件：`~/.llmrelay/llmrelay.pid`
- 后台日志文件：`~/.llmrelay/llmrelay.log`

macOS 用户级安装还会使用：

- 安装目标：`~/Library/Application Support/llmrelay/bin/llmrelay`
- 命令链接：`~/.local/bin/llmrelay`
- LaunchAgent：`~/Library/LaunchAgents/com.yuanboshe.llmrelay.plist`

## config.toml

```toml
listen_addr = "127.0.0.1:18080"

[upstream]
base_url = "https://api.example.test/v1"
api_key_source = "inline"
api_key_env = ""
api_key = ""

[tunnel]
enabled = false
ssh_host = ""
ssh_user = ""
ssh_port = "22"
remote_host = "127.0.0.1"
remote_port = "18080"
```

## listen_addr

`listen_addr` 决定 `llmrelay serve` 监听的地址。

- 只允许本机访问：`127.0.0.1:18080`
- 允许局域网访问：`0.0.0.0:18080`
- 只绑定某个局域网 IP：`192.168.1.10:18080`

暴露到局域网时，请确认 relay token 足够随机，并检查本机防火墙。

## upstream

`upstream.base_url` 是上游 provider 的基础 URL。`llmrelay` 会避免把 base URL 和请求路径拼接成 `/v1/v1/...`。

API key 来源支持：

- `inline`：保存在本地 `config.toml`
- `env`：从环境变量读取

脚本化配置示例：

```sh
llmrelay config set upstream.base_url https://api.example.test/v1
llmrelay config set upstream.api_key
llmrelay test upstream
```

使用环境变量时，确保后台服务启动环境也能读取该变量。

## tunnel

`[tunnel]` 用于远程服务器入口。启用后，`llmrelay serve` 会启动系统 OpenSSH `ssh -R` 进程，并在断开后自动重连。

```toml
[tunnel]
enabled = true
ssh_host = "relay-server"
ssh_user = "ubuntu"
ssh_port = "22"
remote_host = "127.0.0.1"
remote_port = "18080"
```

SSH 配置、私钥、ssh-agent、known_hosts 和 ProxyJump 都复用系统 OpenSSH 行为。

## 常用命令

```sh
llmrelay config show
llmrelay config validate
llmrelay doctor
llmrelay status
llmrelay logs --tail 50
```
