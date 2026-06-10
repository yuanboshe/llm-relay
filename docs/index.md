<script setup>
import { onMounted } from "vue";
import { useRouter } from "vitepress";

const router = useRouter();

onMounted(() => {
  const languages = Array.isArray(navigator.languages) && navigator.languages.length > 0
    ? navigator.languages
    : [navigator.language || ""];
  const prefersChinese = languages.some((language) =>
    language.toLowerCase().startsWith("zh"),
  );

  router.go(prefersChinese ? "/zh/" : "/en/");
});
</script>

# llm-relay Documentation

Choose a documentation home:

- [English documentation](./en/)
- [中文文档](./zh/)
