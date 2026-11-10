<script setup>
import { onMounted, onBeforeUnmount } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { LxErrorPage } from "@dativa-lv/lx-ui";

// Shown when the app has loaded but the server doesn't answer the session check
// (router/events.js). "Try again" repeats it; so does coming back online.
const i18n = useI18n();
const route = useRoute();
const router = useRouter();

function retry() {
  const target = typeof route.query.redirect === "string" ? route.query.redirect : "/";
  router.replace(target);
}

onMounted(() => window.addEventListener("online", retry));
onBeforeUnmount(() => window.removeEventListener("online", retry));
</script>

<template>
  <LxErrorPage
    kind="500"
    :title="i18n.t('pages.offline.title')"
    :description="i18n.t('pages.offline.description')"
    :action-definitions="[{ id: 'retry', name: i18n.t('pages.offline.retry'), icon: 'refresh', kind: 'primary' }]"
    @action-click="retry"
  />
</template>
