<script setup>
// Replaces the plain success/error banner with LxInfoBox. Purely presentational — no
// value to sync, no fallback element to hide beyond the plain banner itself, which
// stays visible if this never mounts.
//
// LxInfoBox takes no default slot, only label/description strings, so the structured
// "which trips conflict" list a booking clash carries (see View.Conflicts) is left as
// plain HTML underneath it rather than flattened into one description string — losing
// that structure to fit the component's API would make the one case where this
// message matters most (a dispatcher deciding what to change) harder to read, not
// easier.
import { LxInfoBox } from "@dativa-lv/lx-ui";

defineProps({
  variant: { type: String, required: true }, // "success" | "error"
  label: { type: String, required: true },
});
</script>

<template>
  <LxInfoBox :variant="variant" :label="label" />
</template>
