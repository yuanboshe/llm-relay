# llm-relay

`llm-relay` 是一个本地或服务器端运行的 LLM API relay 工具。它把客户端发来的 relay token 校验通过后，替换为上游 provider 的 API key，再把请求转发到配置的 upstream `base_url`。

它适合把一台受控机器能够访问的 upstream LLM 能力，安全地提供给远程客户端或同一局域网内的其他客户端。

## 当前能力

- 安装后的命令名统一为 `llmrelay`。
- 支持 `setup` 交互式配置单 upstream 并创建 relay token。
- 支持 `config show`、`config validate`、`config set <key> [value]`。
- 支持 relay token 的 create、list、show、enable、disable、delete 和 rotate。
- 支持真实 HTTP relay：token 鉴权、allowed paths、Authorization 替换、streaming / SSE 转发。
- 支持局域网入口：通过 `listen_addr` 暴露给 LAN client。
- 支持远程入口：推荐 Cloudflare Tunnel，也保留高级 SSH reverse tunnel。
- 支持 `start`、`stop`、`restart`、`status`、`logs` 后台服务管理。

## 当前非目标

- 不承诺完整覆盖 OpenAI-compatible 或 Anthropic Messages API 的所有协议语义。
- 暂不实现 JSONL access log。
- 暂不实现 usage、cached token 统计、限额或限流。
- 暂不实现 Dashboard、多 provider 管理、复杂权限模型或 macOS Keychain。

## 推荐阅读顺序

1. [快速开始](./quickstart.md)
2. [命令手册](./commands.md)
3. [部署闭环](./deploy.md)
4. [配置](./config.md)
5. [Token 管理](./tokens.md)
6. [安全边界](./security.md)
7. [故障排查](./troubleshooting.md)
8. [本地文档站](./docs-site.md)
