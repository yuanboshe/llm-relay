# 源码编译

本页说明如何从源码生成 `llmrelay` 单文件 release 二进制。以下命令都在公开仓库根目录执行。

## 默认构建

```sh
make build
```

`make build` 会构建默认 release 目标，并把产物写入 `dist/`：

```text
dist/llmrelay-linux-amd64
dist/llmrelay-linux-arm64
dist/llmrelay-windows-amd64.exe
dist/llmrelay-darwin-amd64
dist/llmrelay-darwin-arm64
dist/SHA256SUMS
```

## 构建目标

常用目标如下：

```sh
make build-local
make build-linux-amd64
make build-linux-arm64
make build-windows-amd64
make build-darwin-amd64
make build-darwin-arm64
make clean
```

`make build-local` 只构建当前机器对应的 `GOOS/GOARCH`。平台目标只构建对应平台的单个二进制。`make clean` 删除 `dist/` 和 `coverage.out`。

## 版本参数

构建参数通过 Make 变量传入：

```sh
make build VERSION=v0.1.0
make build VERSION=v0.1.0 COMMIT=abc1234 BUILD_DATE=2026-06-11T00:00:00Z
```

参数规则：

- `VERSION` 默认是 `v0.0.0`。正式 release 构建应显式设置，例如 `VERSION=v0.1.0`。
- `COMMIT` 默认是当前 Git 短 commit。只有在源码不来自 Git checkout 或需要固定显示值时才需要覆盖。
- `BUILD_DATE` 为空时由构建脚本写入当前 UTC 时间。手动指定时建议使用 RFC3339 格式，例如 `2026-06-11T00:00:00Z`。

## 编译和安装

`make build` 只负责生成二进制，不会安装到用户目录，也不会写入配置文件。项目不提供 `make install` 作为正式安装入口。

如果要安装本地编译出的二进制，使用安装脚本的 `--local` 参数：

```sh
sh ./scripts/install.sh --local ./dist/llmrelay-darwin-arm64
```
