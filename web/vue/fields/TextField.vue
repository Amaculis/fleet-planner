<script setup>
// Enhances a plain <input type="text"|"email"|"tel"> with LxTextInput. See main.js for
// the mount/fallback contract every field island shares.
import { ref, watch } from "vue";
import { LxTextInput } from "@dativa-lv/lx-ui";

const props = defineProps({
  initialValue: { type: String, default: "" },
  placeholder: { type: String, default: null },
  required: { type: Boolean, default: false },
  maxlength: { type: [String, Number], default: null },
  uppercase: { type: Boolean, default: false }, // bus plates are stored normalised
  labelId: { type: String, default: null },
});
const emit = defineEmits(["change"]);

const value = ref(props.initialValue);
watch(value, (next) => emit("change", next ?? ""));
</script>

<template>
  <LxTextInput
    v-model="value"
    :placeholder="placeholder"
    :required="required"
    :maxlength="maxlength"
    :uppercase="uppercase"
    :label-id="labelId"
  />
</template>
