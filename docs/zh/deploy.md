# 部署闭环

`llm-relay` 的部署目标是把 upstream API key 留在受控的 relay host 上。Cloudflare、remote server、reverse proxy、SSH tunnel 和客户端都只接触 relay token，不保存 upstream API key。

## 局域网入口

局域网入口适合让同一 LAN 内的客户端访问 relay host。

```text
LAN client
  base_url = http://relay-host-lan-ip:18080
  api_key = llmr_xxx
        ↓
relay host llmrelay
  validate relay token
  replace Authorization
        ↓
upstream LLM provider
```

relay host 的 `config.toml` 可以监听所有网卡：

```toml
listen_addr = "0.0.0.0:18080"
```

LAN client 使用：

```text
base_url = http://relay-host-lan-ip:18080
api_key = llmr_xxx
```

第一版局域网入口使用 HTTP + relay token。如果需要局域网 HTTPS，可以后续用本机 Caddy 或内置 TLS 单独规划。

## Cloudflare Tunnel 入口

Cloudflare Tunnel 是推荐远程入口，适合没有自有服务器或不想配置 SSH 的用户。

```text
remote client
  base_url = https://llm.example.test
  api_key = llmr_xxx
        ↓
Cloudflare Public Hostname
        ↓
cloudflared on relay host
        ↓
relay host llmrelay
  validate relay token
  replace Authorization
        ↓
upstream LLM provider
```

Cloudflare Public Hostname 的 Service URL 指向本机 relay：

```text
http://127.0.0.1:18080
```

relay host 上运行：

```sh
llmrelay setup
llmrelay start
llmrelay test remote-client https://llm.example.test
```

Cloudflare Tunnel 不使用 `[tunnel]` SSH 配置：

```toml
[tunnel]
enabled = false
```

## 高级入口：远程服务器和 SSH reverse tunnel

远程入口适合让远程客户端通过 HTTPS 访问 relay host。remote server 只提供 HTTPS 和透明转发，不保存 upstream API key，也不做 relay token 鉴权。

```text
remote client
  base_url = https://relay.example.test
  api_key = llmr_xxx
        ↓
Caddy on remote server
        ↓
SSH reverse tunnel
        ↓
relay host llmrelay
  validate relay token
  replace Authorization
        ↓
upstream LLM provider
```

在 relay host 的 `config.toml` 中启用 tunnel：

```toml
[tunnel]
enabled = true
ssh_host = "relay-server"
ssh_user = "ubuntu"
ssh_port = "22"
remote_host = "127.0.0.1"
remote_port = "18080"
```

`llmrelay` 使用系统 OpenSSH 建立 reverse tunnel，等价于：

```sh
ssh -N -T -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3 \
  -p 22 \
  -R 127.0.0.1:18080:127.0.0.1:18080 \
  ubuntu@relay-server
```

remote server 上可以用 Caddy 暴露 HTTPS：

```text
relay.example.test {
    reverse_proxy 127.0.0.1:18080
}
```

remote client 使用：

```text
base_url = https://relay.example.test
api_key = llmr_xxx
```

## 验证清单

- `llmrelay doctor` 不输出 upstream API key 或 relay token 明文。
- `llmrelay test upstream` 在 relay host 上成功；如果 upstream 返回模型 ID，输出中会展示部分模型列表。
- `llmrelay start` 后 `llmrelay status` 显示后台服务运行。
- `llmrelay test <key-id>` 在 relay host 上成功，并展示通过该 token 获取到的部分模型列表。
- `llmrelay logs --tail 50` 能看到本地监听和 tunnel 启动信息。
- remote server 只连接 `127.0.0.1:<remote_port>`，不保存 upstream API key。
- 客户端使用 relay token 调用 `/v1/models` 成功。
- upstream provider 不会收到 `llmr_` relay token，只会收到真实 upstream API key。
