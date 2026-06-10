import { defineConfig } from "vitepress";

const docsBase = process.env.LLM_RELAY_DOCS_BASE || "/llm-relay/";

const zhGuide = [
  { text: "快速开始", link: "/zh/quickstart" },
  { text: "源码编译", link: "/zh/build" },
  { text: "部署闭环", link: "/zh/deploy" },
];

const zhReference = [
  { text: "命令手册", link: "/zh/commands" },
  { text: "配置", link: "/zh/config" },
  { text: "Token 管理", link: "/zh/tokens" },
  { text: "安全边界", link: "/zh/security" },
  { text: "故障排查", link: "/zh/troubleshooting" },
  { text: "本地文档站", link: "/zh/docs-site" },
];

const enGuide = [
  { text: "Quick Start", link: "/en/quickstart" },
  { text: "Build from Source", link: "/en/build" },
  { text: "Deployment Loop", link: "/en/deploy" },
];

const enReference = [
  { text: "Command Reference", link: "/en/commands" },
  { text: "Configuration", link: "/en/config" },
  { text: "Token Management", link: "/en/tokens" },
  { text: "Security Boundaries", link: "/en/security" },
  { text: "Troubleshooting", link: "/en/troubleshooting" },
  { text: "Local Docs Site", link: "/en/docs-site" },
];

const sharedThemeConfig = {
  search: {
    provider: "local" as const,
  },
  socialLinks: [
    { icon: "github" as const, link: "https://github.com/yuanboshe/llm-relay" },
  ],
};

export default defineConfig({
  lang: "en-US",
  title: "llm-relay",
  description: "Local or server-side LLM API relay tool",
  base: docsBase,
  cleanUrls: true,
  themeConfig: {
    ...sharedThemeConfig,
  },
  locales: {
    en: {
      label: "English",
      lang: "en-US",
      link: "/en/",
      title: "llm-relay",
      description: "Local or server-side LLM API relay tool",
      themeConfig: {
        ...sharedThemeConfig,
        nav: [
          { text: "Home", link: "/en/" },
          { text: "Quick Start", link: "/en/quickstart" },
          { text: "Deploy", link: "/en/deploy" },
          { text: "Reference", link: "/en/config" },
        ],
        sidebar: {
          "/en/": [
            {
              text: "Start",
              items: enGuide,
            },
            {
              text: "Reference",
              items: enReference,
            },
          ],
        },
      },
    },
    zh: {
      label: "简体中文",
      lang: "zh-CN",
      link: "/zh/",
      title: "llm-relay",
      description: "本地或服务器端 LLM API relay 工具",
      themeConfig: {
        ...sharedThemeConfig,
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
              items: zhGuide,
            },
            {
              text: "参考",
              items: zhReference,
            },
          ],
        },
      },
    },
  },
});
