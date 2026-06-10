# 本地文档站

本项目的公开文档站使用 VitePress。以下命令都在公开仓库根目录执行。

## 安装依赖

首次启动前安装 Node.js 依赖：

```sh
npm ci
```

## 开发模式

开发模式会启动本地测试站点，并在编辑文档后自动刷新。

```sh
npm run docs:dev
```

默认地址通常是：

```text
http://localhost:5173/llm-relay/
```

如果端口被占用，VitePress 会自动选择下一个可用端口，请以终端输出为准。

## 构建

发布前先构建静态站点：

```sh
npm run docs:build
```

构建产物位于：

```text
docs/.vitepress/dist
```

该目录是生成产物，不提交到 Git。

## 预览构建产物

构建后可以用 preview 检查最终静态站点：

```sh
npm run docs:preview
```

默认访问：

```text
http://localhost:4173/llm-relay/
```

如果需要指定 host 或端口：

```sh
npm run docs:preview -- --host 127.0.0.1 --port 4173
```

## GitHub Pages 路径

文档站按 GitHub Pages 项目页部署，默认 base path 是：

```text
/llm-relay/
```

本地打开页面时也应访问带 `/llm-relay/` 前缀的路径，例如：

```text
http://localhost:4173/llm-relay/zh/quickstart
```
