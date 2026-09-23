<script setup>
import { LxErrorPage } from "@dativa-lv/lx-ui";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";

const router = useRouter();
const i18n = useI18n();

function action(actionName) {
  if (actionName === "dashboard") {
    router.push({ name: "dashboard" });
    return;
  }
  router.go(-1);
}
</script>

<template>
  <LxErrorPage
    kind="404"
    :title="i18n.t('pages.notFound.title')"
    :description="i18n.t('pages.notFound.description')"
    :action-definitions="[
      { id: 'back', name: i18n.t('pages.error.goBack'), icon: 'undo' },
      { id: 'dashboard', name: i18n.t('pages.error.goHome'), icon: 'dashboard', kind: 'secondary' },
    ]"
    @action-click="action"
  />
</template>
