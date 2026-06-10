# 故障排查

优先从本机配置、upstream 连通性、后台服务状态和 tunnel 状态四个层面排查。

## 配置路径

查看配置：

```sh
llmrelay config show
```

校验配置：

```sh
llmrelay config validate
```

如果使用自定义目录，确认环境变量一致：

```sh
echo "$LLMRELAY_HOME"
```

## upstream 连通性

在 relay host 上测试 upstream：

```sh
llmrelay config test --path /v1/models
```

如果失败，检查：

- `upstream.base_url` 是否正确
- API key 来源是否正确
- `api_key_source = "env"` 时环境变量是否存在
- upstream provider 是否允许当前 relay host 访问

## 本地服务

查看状态和日志：

```sh
llmrelay status
llmrelay logs --tail 100
```

排查时可以前台运行：

```sh
llmrelay serve
```

常见现象：

- 无 Authorization：返回 401
- 非 `llmr_` bearer token：返回 401
- 未知 token：返回 401
- 禁用 token：返回 403
- 非 allowed path：返回 404 或 403

## tunnel

如果远程入口不可用，先确认系统 OpenSSH 可用：

```sh
ssh relay-server
```

再检查 remote server 上 Caddy 是否能访问 tunnel 绑定端口：

```sh
curl http://127.0.0.1:18080/v1/models
```

如果 Caddy 返回 502：

- 查看 `llmrelay status`
- 查看 `llmrelay logs --tail 100`
- 确认 `remote_port` 与 Caddy 反代端口一致
- 确认 remote server 的 sshd 允许 reverse tunnel

## 局域网入口

LAN client 连接失败时检查：

- `listen_addr` 是否是 `0.0.0.0:<port>` 或 relay host 的局域网 IP
- 本机防火墙是否放行该端口
- LAN client 的 `base_url` 是否指向 relay host
- LAN client 使用的是 relay token，而不是 upstream API key

## 仍未实现的能力

以下能力还不是当前排查入口：

- JSONL access log
- `usage.sqlite`
- cached token 统计
- 按 relay token 限额或限流
