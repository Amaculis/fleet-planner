<script setup>
import { watch } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";

const i18n = useI18n();
const route = useRoute();

watch(
  route,
  () => {
    let title = i18n.t("title.prefix");
    if (typeof route.meta.title === "function") {
      title += route.meta.title(i18n);
    } else {
      title = i18n.t(route.meta?.title?.toString() || "title.default");
    }
    document.title = title;
  },
  { immediate: true }
);
</script>

<template>
  <router-view />
</template>
