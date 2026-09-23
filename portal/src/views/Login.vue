<script setup>
import { ref } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxTextInput, LxButton, LxSection, LxRow, LxInfoBox } from "@dativa-lv/lx-ui";
import useAuthStore from "@/stores/auth";
import useErrors from "@/hooks/errors";

const i18n = useI18n();
const router = useRouter();
const route = useRoute();
const auth = useAuthStore();
const errors = useErrors();

const email = ref("");
const password = ref("");
const submitting = ref(false);
const errorMessage = ref("");

async function submit() {
  errorMessage.value = "";
  submitting.value = true;
  try {
    await auth.login(email.value, password.value);
    router.push(route.query.redirect?.toString() || { name: "dashboard" });
  } catch (error) {
    const err = errors.get(error);
    errorMessage.value =
      err.status === 401 ? i18n.t("login.error.invalid") : i18n.t("login.error.generic");
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <div class="lx-region lx-region-primary">
    <LxSection :label="i18n.t('login.title')">
      <LxRow :label="i18n.t('login.email')" required>
        <LxTextInput v-model="email" mask="email" autocomplete="username" @keyup.enter="submit" />
      </LxRow>
      <LxRow :label="i18n.t('login.password')" required>
        <LxTextInput v-model="password" kind="password" autocomplete="current-password" @keyup.enter="submit" />
      </LxRow>

      <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />

      <LxButton :label="i18n.t('login.submit')" kind="primary" :loading="submitting" @click="submit" />
    </LxSection>
  </div>
</template>
