<script setup>
import { ref, computed } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxShell, LxTextInput, LxForm, LxRow, LxInfoBox } from "@dativa-lv/lx-ui";
import useAuthStore from "@/stores/auth";
import useErrors from "@/hooks/errors";
import { useLanguageSwitcher } from "@/hooks/language";
import useShellTexts from "@/hooks/shellTexts";
import { APP_CONFIG } from "@/constants";

const i18n = useI18n();
const router = useRouter();
const route = useRoute();
const auth = useAuthStore();
const errors = useErrors();
const languageSwitcher = useLanguageSwitcher();
const shellTexts = useShellTexts();

const languages = computed(() =>
  APP_CONFIG.supportedLocales.map((id) => ({ id, name: i18n.t(`languages.${id}`) }))
);
const selectedLanguage = computed({
  get: () => languages.value.find((l) => l.id === i18n.locale.value) ?? languages.value[0],
  set: (lang) => languageSwitcher.switchLocale(lang.id),
});

const email = ref("");
const password = ref("");
const submitting = ref(false);
const errorMessage = ref("");
// Set by api.js when a request found the session expired.
const expired = computed(() => route.query.expired === "1");

const actionDefinitions = computed(() => [
  { id: "submit", name: i18n.t("login.submit"), kind: "primary", loading: submitting.value },
]);

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
  <LxShell
    mode="cover"
    :has-cover-logo="false"
    :cover-image="`${APP_CONFIG.publicUrl}imgs/cover.svg`"
    :cover-image-dark="`${APP_CONFIG.publicUrl}imgs/cover.svg`"
    :route-name="route.name?.toString()"
    :system-name="i18n.t('title.fullName')"
    :system-subheader="i18n.t('login.subtitle')"
    :texts="shellTexts"
    has-language-picker
    :languages="languages"
    v-model:selected-language="selectedLanguage"
  >
    <template #coverArea>
      <LxForm
        :show-header="false"
        required-mode="required-asterisk"
        :aria-label="i18n.t('login.title')"
        :action-definitions="actionDefinitions"
        @action-click="submit"
      >
        <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />
        <LxInfoBox v-else-if="expired" variant="warning" :label="i18n.t('login.expired')" />
        <LxRow :label="i18n.t('login.email')" required>
          <LxTextInput v-model="email" mask="email" autocomplete="username" @keyup.enter="submit" />
        </LxRow>
        <LxRow :label="i18n.t('login.password')" required>
          <LxTextInput v-model="password" kind="password" autocomplete="current-password" @keyup.enter="submit" />
        </LxRow>
      </LxForm>
    </template>
  </LxShell>
</template>
