<script setup>
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { LxErrorPage } from "@dativa-lv/lx-ui";

const i18n = useI18n();
const router = useRouter();

function action(actionId) {
  if (actionId === "dashboard") {
    router.push({ name: "dashboard" });
    return;
  }
  router.go(-1);
}
</script>

<template>
  <LxErrorPage
    kind="403"
    :title="i18n.t('pages.forbidden.title')"
    :description="i18n.t('pages.forbidden.description')"
    :action-definitions="[
      { id: 'back', name: i18n.t('pages.error.goBack'), icon: 'undo' },
      { id: 'dashboard', name: i18n.t('pages.error.goHome'), icon: 'dashboard', kind: 'secondary' },
    ]"
    @action-click="action"
  />
</template>
