<script setup>
// Replaces the plain nav bar with LxToolbar. Unlike every field island, this one has
// no native "value" to sync back — a nav bar has no data, only actions — so the
// fallback contract here is simpler: if this fails to mount, main.js never hides the
// plain <nav>, and nothing about it needed JS to begin with.
//
// LxToolbar's own `href` field expects a Vue Router route object (its docs example is
// `{ name: 'info' }') — this app has no client-side router and never will, so `href`
// is not used at all. Navigation is handled entirely in our own actionClick listener
// instead, using the same plain page URLs the fallback <nav> already links to.
import { LxToolbar } from "@dativa-lv/lx-ui";

const props = defineProps({
  // [{ id, name, href, active }] — one entry per visible nav link, built server-side
  // from the exact same role-gated list the plain <nav> already renders.
  items: { type: Array, required: true },
  // id of the real, hidden <form method="post" action="/logout"> — logout is a
  // state-changing request and must stay a CSRF-protected POST, never a GET
  // navigation, so it is submitted programmatically rather than treated as a link.
  logoutFormId: { type: String, required: true },
});

const actionDefinitions = props.items.map((item) => ({
  id: item.id,
  name: item.name,
  kind: item.active ? "primary" : "ghost",
}));

function handleActionClick(id) {
  if (id === "logout") {
    document.getElementById(props.logoutFormId)?.requestSubmit();
    return;
  }
  const item = props.items.find((i) => i.id === id);
  if (item?.href) window.location.assign(item.href);
}
</script>

<template>
  <LxToolbar :action-definitions="actionDefinitions" @action-click="handleActionClick" />
</template>
