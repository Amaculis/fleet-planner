<script setup>
// Replaces one real <button> with LxButton. Scoped deliberately narrow: only the
// single primary "Save" submit button per form, and the top-of-list "Add …" actions —
// never a per-row table button (Edit/Delete repeated per row), where mounting a
// separate Vue app per row would mean dozens of createApp() calls on one page for no
// real benefit. See docs/lx-ui-integration.md for the full scoping rationale.
//
// The real button is never replicated in Vue — it stays exactly where it was, hidden
// on success. Clicking the island's button finds it by id and calls .click() on it,
// which — for a real <button type="submit"> inside a <form> — triggers the same
// native submission a direct click would. No new submission path, no duplicated
// validation, nothing this component could get subtly wrong that the native fallback
// wouldn't already catch.
import { LxButton } from "@dativa-lv/lx-ui";

const props = defineProps({
  label: { type: String, required: true },
  kind: { type: String, default: "primary" }, // "primary" | "secondary" | "ghost"
  destructive: { type: Boolean, default: false },
  fallbackId: { type: String, required: true },
});

function handleClick() {
  document.getElementById(props.fallbackId)?.click();
}
</script>

<template>
  <LxButton :label="label" :kind="kind" :destructive="destructive" @click="handleClick" />
</template>
