<script setup>
import { watch, defineAsyncComponent } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { APP_CONFIG } from "@/constants";

const i18n = useI18n();
const route = useRoute();

// Dynamic import, not a static one: this whole demo/ tree (including
// axios-mock-adapter) must never be fetched by the real build the Go server
// embeds, only by the GitHub Pages demo build where APP_CONFIG.demo is true. A
// static import would bundle it into every build's main chunk regardless.
const DemoBanner = APP_CONFIG.demo ? defineAsyncComponent(() => import("@/demo/DemoBanner.vue")) : null;

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
  <component :is="DemoBanner" v-if="DemoBanner" />
  <router-view />
</template>
