<script setup>
import { onMounted } from "vue";
import { useRouter } from "vitepress";

const router = useRouter();

onMounted(() => {
  router.go("/zh/");
});
</script>

# llm-relay 文档

正在进入 [中文文档](./zh/)。

当前文档站先提供中文版本。英文版本会在中文内容稳定后补齐。
