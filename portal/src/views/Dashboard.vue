<script setup>
import { LxWidget, LxRow } from "@dativa-lv/lx-ui";
import { useI18n } from "vue-i18n";
import useAuthStore from "@/stores/auth";

const i18n = useI18n();
const auth = useAuthStore();
</script>

<template>
  <LxWidget :label="i18n.t('pages.dashboard.title')">
    <LxRow :label="i18n.t('login.email')">{{ auth.session?.email }}</LxRow>
    <LxRow :label="i18n.t('pages.dashboard.role')">{{ auth.session?.role }}</LxRow>
  </LxWidget>
</template>
