# Local Documentation Site

This project's public documentation site uses VitePress. Run all commands from the public repository root.

This page only describes documentation site development, build, and preview. For source builds of the project binary, see [Build from Source](./build.md).

## Install Dependencies

Install Node.js dependencies before starting the site for the first time:

```sh
npm ci
```

## Development Mode

Development mode starts a local test site and refreshes automatically after documentation edits.

```sh
npm run docs:dev
```

The default address is usually:

```text
http://localhost:5173/llm-relay/
```

If the port is occupied, VitePress automatically chooses the next available port. Use the terminal output as the source of truth.

## Build

Build the static site before publishing:

```sh
npm run docs:build
```

The build output is written to:

```text
docs/.vitepress/dist
```

This directory is generated output and is not committed to Git.

## Preview Build Output

After building, preview the final static site:

```sh
npm run docs:preview
```

The default address is:

```text
http://localhost:4173/llm-relay/
```

To specify host or port:

```sh
npm run docs:preview -- --host 127.0.0.1 --port 4173
```

## GitHub Pages Path

The documentation site is deployed as a GitHub Pages project site. The default base path is:

```text
/llm-relay/
```

When opening local pages, include the `/llm-relay/` prefix, for example:

```text
http://localhost:4173/llm-relay/en/quickstart
```
