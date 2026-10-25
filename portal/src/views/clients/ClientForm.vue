<script setup>
import { ref, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxForm, LxSection, LxRow, LxTextInput, LxTextArea, LxToggle, LxInfoBox } from "@dativa-lv/lx-ui";
import { getClient, createClient, updateClient } from "@/services/clients";
import useErrors from "@/hooks/errors";
import useFormTexts from "@/hooks/formTexts";
import useFormActions from "@/hooks/formActions";
import useFormValidation from "@/hooks/formValidation";

const i18n = useI18n();
const route = useRoute();
const router = useRouter();
const errors = useErrors();
const formTexts = useFormTexts();

const isNew = computed(() => route.name === "clientNew");
const id = computed(() => route.params.id);

// Mirrors internal/service/validate.go validateClient.
const PHONE_PATTERN = /^\+?[0-9 ()-]{5,32}$/;
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const MAX_NAME = 200;
const MAX_EMAIL = 254;
const MAX_ADDRESS = 500;
const MAX_NOTES = 2000;

const name = ref("");
const contactPerson = ref("");
const phone = ref("");
const email = ref("");
const address = ref("");
const notes = ref("");
const isActive = ref(true);
const anonymized = ref(false);

const loadingClient = ref(false);
const saving = ref(false);
const errorMessage = ref("");

const { invalidProps, validate } = useFormValidation(() => {
  const e = {};
  if (!name.value.trim()) e.name = i18n.t("validation.required");
  else if (name.value.trim().length > MAX_NAME) e.name = i18n.t("validation.tooLong", { max: MAX_NAME });

  if (contactPerson.value.trim().length > MAX_NAME) e.contactPerson = i18n.t("validation.tooLong", { max: MAX_NAME });
  if (phone.value.trim() && !PHONE_PATTERN.test(phone.value.trim())) e.phone = i18n.t("validation.invalidPhone");
  if (email.value.trim()) {
    if (email.value.trim().length > MAX_EMAIL) e.email = i18n.t("validation.tooLong", { max: MAX_EMAIL });
    else if (!EMAIL_PATTERN.test(email.value.trim())) e.email = i18n.t("validation.invalidEmail");
  }
  if (address.value.length > MAX_ADDRESS) e.address = i18n.t("validation.tooLong", { max: MAX_ADDRESS });
  if (notes.value.length > MAX_NOTES) e.notes = i18n.t("validation.tooLong", { max: MAX_NOTES });
  return e;
});

async function load() {
  if (isNew.value) return;
  loadingClient.value = true;
  try {
    const client = (await getClient(id.value)).data;
    name.value = client.name;
    contactPerson.value = client.contactPerson ?? "";
    phone.value = client.phone ?? "";
    email.value = client.email ?? "";
    address.value = client.address ?? "";
    notes.value = client.notes ?? "";
    isActive.value = client.isActive;
    anonymized.value = !!client.anonymized;
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    loadingClient.value = false;
  }
}

async function save() {
  errorMessage.value = "";
  if (!validate()) return;

  saving.value = true;
  const payload = {
    name: name.value.trim(),
    contactPerson: contactPerson.value.trim() || null,
    phone: phone.value.trim() || null,
    email: email.value.trim() || null,
    address: address.value.trim() || null,
    notes: notes.value.trim() || null,
    isActive: isActive.value,
  };
  try {
    if (isNew.value) {
      await createClient(payload);
    } else {
      await updateClient(id.value, payload);
    }
    router.push({ name: "clients" });
  } catch (error) {
    errorMessage.value = i18n.t(errors.get(error).message);
  } finally {
    saving.value = false;
  }
}

const { actionDefinitions, onAction } = useFormActions({
  saving,
  disabled: computed(() => loadingClient.value || anonymized.value),
  onSave: save,
  onCancel: () => router.push({ name: "clients" }),
});

onMounted(load);
</script>

<template>
  <LxForm
    :column-count="2"
    required-mode="required-asterisk"
    :show-header="false"
    :aria-label="isNew ? i18n.t('pages.clients.new') : i18n.t('pages.clients.edit')"
    :texts="formTexts"
    :action-definitions="actionDefinitions"
    @action-click="onAction"
  >
    <LxInfoBox v-if="errorMessage" variant="error" :label="errorMessage" />
    <LxInfoBox v-if="anonymized" variant="info" :label="i18n.t('clientForm.erased')" />

    <LxSection :label="i18n.t('clientForm.details')" :description="i18n.t('clientForm.detailsDescription')">
      <LxRow :label="i18n.t('fields.clientName')" required>
        <LxTextInput v-model="name" :maxlength="MAX_NAME" v-bind="invalidProps('name')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.contactPerson')">
        <LxTextInput v-model="contactPerson" :maxlength="MAX_NAME" v-bind="invalidProps('contactPerson')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.phone')">
        <LxTextInput v-model="phone" v-bind="invalidProps('phone')" :helper-text="i18n.t('driverForm.phoneHint')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.email')">
        <LxTextInput v-model="email" :maxlength="MAX_EMAIL" v-bind="invalidProps('email')" />
      </LxRow>
      <LxRow :label="i18n.t('fields.address')" :column-span="2">
        <LxTextInput v-model="address" :maxlength="MAX_ADDRESS" v-bind="invalidProps('address')" />
      </LxRow>
    </LxSection>

    <LxSection v-if="!isNew" :label="i18n.t('clientForm.status')" :description="i18n.t('clientForm.statusDescription')">
      <LxRow :label="i18n.t('fields.isActive')">
        <LxToggle v-model="isActive" />
      </LxRow>
    </LxSection>

    <LxSection :label="i18n.t('clientForm.notesSection')" :description="i18n.t('clientForm.notesHint')">
      <LxRow :label="i18n.t('fields.notes')" :column-span="2">
        <LxTextArea v-model="notes" :rows="4" :maxlength="MAX_NOTES" v-bind="invalidProps('notes')" />
      </LxRow>
    </LxSection>
  </LxForm>
</template>
