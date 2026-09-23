<script setup>
// Enhances a plain <select> with LxValuePicker in its "dropdown" variant.
import { ref, watch } from "vue";
import { LxValuePicker } from "@dativa-lv/lx-ui";

const props = defineProps({
  // [{ id: "active", name: "In service" }, ...] — built server-side from the same
  // enum/list the plain <option> elements already render, so the two never drift.
  items: { type: Array, required: true },
  initialValue: { type: [String, Number], default: null },
  nullable: { type: Boolean, default: false },
  required: { type: Boolean, default: false },
  labelId: { type: String, default: null },
});
const emit = defineEmits(["change"]);

const value = ref(props.initialValue);
watch(value, (next) => emit("change", next ?? ""));
</script>

<template>
  <LxValuePicker
    v-model="value"
    :items="items"
    variant="dropdown"
    selection-kind="single"
    :nullable="nullable"
    :required="required"
    :label-id="labelId"
  />
</template>
