# Token 管理

客户端使用 relay token 访问 `llmrelay`。`llmrelay` 校验 token 后，会把请求中的 `Authorization: Bearer llmr_xxx` 替换为 upstream API key。

relay token 不能发送给上游 provider。

## 创建

```sh
llmrelay token create remote-client
```

输出中的 relay token 需要复制给对应客户端：

```text
key-id: remote-client
relay token: llmr_xxx
```

## 列表与查看

```sh
llmrelay token list
llmrelay token show remote-client
```

`list` 和 `show` 会输出明文 relay token，便于复制到客户机。命令输出应按敏感凭据处理。

## 禁用与启用

```sh
llmrelay token disable remote-client
llmrelay token enable remote-client
```

禁用后的 token 会被拒绝。未知 token 返回 401，禁用 token 返回 403。

## 删除与轮换

```sh
llmrelay token rotate remote-client
llmrelay token delete remote-client
```

轮换会生成新的 token，并让旧 token 失效。删除会移除该 `key_id` 的记录。

## tokens.json

`tokens.json` 由 `llmrelay token ...` 命令管理。当前版本会保存 relay token 明文；内部可以保留 hash 字段用于旧记录兼容，但用户命令面以明文 token 为准。

示例：

```json
[
  {
    "key_id": "remote-client",
    "token": "llmr_xxx",
    "token_hash": "sha256:<hex>",
    "created_at": "2026-06-10T00:00:00Z",
    "rotated_at": "",
    "enabled": true
  }
]
```

`tokens.json` 包含可直接使用的 relay credential。不要同步、泄漏或提交这个文件。
