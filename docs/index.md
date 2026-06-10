<script setup>
import { onMounted } from "vue";
import { withBase } from "vitepress";

onMounted(() => {
  const languages = Array.isArray(navigator.languages) && navigator.languages.length > 0
    ? navigator.languages
    : [navigator.language || ""];
  const prefersChinese = languages.some((language) =>
    language.toLowerCase().startsWith("zh"),
  );

  window.location.replace(withBase(prefersChinese ? "/zh/" : "/en/"));
});
</script>

# llm-relay Documentation

Choose a documentation home:

- [English documentation](./en/)
- [中文文档](./zh/)
