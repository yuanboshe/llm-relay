# 安全边界

`llm-relay` 的核心边界是：客户端只持有 relay token，upstream API key 只保存在 relay host 或它受控的配置来源中。

## 请求链路

```text
client
  Authorization: Bearer llmr_xxx
        ↓
llmrelay
  validate relay token
  resolve key_id
  replace Authorization with upstream API key
        ↓
upstream provider
```

## 不应保存 upstream API key 的位置

- remote server
- Caddy 或其他 reverse proxy
- SSH tunnel
- remote client
- LAN client

remote server 和 reverse proxy 只做透明转发，不做业务鉴权。

## 需要保护的本地文件

- `~/.llmrelay/config.toml`：可能包含 upstream API key
- `~/.llmrelay/tokens.json`：包含可直接使用的 relay token 明文
- `~/.llmrelay/llmrelay.log`：运行日志不应包含密钥，但仍应按敏感运行日志处理

不要把这些文件提交到 Git，也不要同步到不可信位置。

## 日志边界

当前版本不实现 JSONL access log。运行日志用于观察服务启动、监听、tunnel 启动和重连事件。

设计边界：

- 不记录请求体
- 不记录 Authorization 明文
- 不把 relay token 发送给 upstream provider
- 后续 access log 只能记录必要元数据，例如时间、`key_id`、method、path、status、latency 和 bytes_out

## 局域网暴露

当 `listen_addr = "0.0.0.0:18080"` 时，局域网内其他机器可以访问 relay。启用前请确认：

- relay token 已按客户端分开创建
- 不再使用的 token 已 disable、rotate 或 delete
- 本机防火墙只放行必要网络范围
- 客户端配置使用 relay token，不使用 upstream API key

## 远程入口

远程入口建议使用：

- remote server 上的 HTTPS / Caddy
- SSH reverse tunnel
- remote bind `127.0.0.1:<remote_port>`

remote server 不保存 upstream API key，也不需要保存 relay token。
