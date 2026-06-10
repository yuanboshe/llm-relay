import { defineConfig } from "vitepress";

const docsBase = process.env.LLM_RELAY_DOCS_BASE || "/llm-relay/";

const guide = [
  { text: "快速开始", link: "/zh/quickstart" },
  { text: "部署闭环", link: "/zh/deploy" },
];

const reference = [
  { text: "命令手册", link: "/zh/commands" },
  { text: "配置", link: "/zh/config" },
  { text: "Token 管理", link: "/zh/tokens" },
  { text: "安全边界", link: "/zh/security" },
  { text: "故障排查", link: "/zh/troubleshooting" },
  { text: "本地文档站", link: "/zh/docs-site" },
];

export default defineConfig({
  lang: "zh-CN",
  title: "llm-relay",
  description: "本地或服务器端 LLM API relay 工具",
  base: docsBase,
  cleanUrls: true,
  themeConfig: {
    nav: [
      { text: "首页", link: "/zh/" },
      { text: "快速开始", link: "/zh/quickstart" },
      { text: "部署", link: "/zh/deploy" },
      { text: "参考", link: "/zh/config" },
    ],
    sidebar: {
      "/zh/": [
        {
          text: "开始",
          items: guide,
        },
        {
          text: "参考",
          items: reference,
        },
      ],
    },
    search: {
      provider: "local",
    },
    socialLinks: [
      { icon: "github", link: "https://github.com/yuanboshe/llm-relay" },
    ],
  },
});
