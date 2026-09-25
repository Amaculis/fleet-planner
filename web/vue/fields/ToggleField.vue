<script setup>
// Enhances a plain <input type="checkbox"> with LxToggle.
import { ref, watch } from "vue";
import { LxToggle } from "@dativa-lv/lx-ui";

const props = defineProps({
  initialChecked: { type: Boolean, default: false },
  label: { type: String, default: null },
  labelId: { type: String, default: null },
});
const emit = defineEmits(["change"]);

const value = ref(props.initialChecked);
watch(value, (next) => emit("change", !!next));
</script>

<template>
  <LxToggle v-model="value" :label="label" :label-id="labelId" />
</template>
