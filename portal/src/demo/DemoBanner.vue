<script setup>
import { computed } from "vue";
import { LxContentSwitcher, LxButton } from "@dativa-lv/lx-ui";
import { demoRole } from "@/demo/mockApi";

const roleItems = [
  { id: "admin", name: "Admin" },
  { id: "dispatcher", name: "Dispatcher" },
  { id: "driver", name: "Driver" },
];

const role = computed({
  get: () => demoRole.value,
  set: (value) => {
    // A hard reload, not a route push: every view loads its own data once on
    // mount (Dashboard.vue's getMyTrips(), TripList.vue's getTrips(), ...), keyed
    // off the role active at that moment — switching role while already sitting
    // on /dashboard is a no-op navigation to vue-router (same route), so nothing
    // would ever re-fetch under the new role without this. sessionStorage carries
    // the choice across the reload (see mockApi.js's demoRole initializer).
    sessionStorage.setItem("demoRole", value);
    window.location.reload();
  },
});

function reset() {
  sessionStorage.removeItem("demoRole");
  window.location.reload();
}
</script>

<template>
  <div class="demo-banner">
    <span class="demo-banner-label">
      Demo — fake data, nothing is saved.
      <a href="https://github.com/Amaculis/fleet-planner" target="_blank" rel="noopener">Source</a>
    </span>
    <div class="demo-banner-controls">
      <span class="demo-banner-role-label">Viewing as:</span>
      <LxContentSwitcher v-model="role" :items="roleItems" />
      <LxButton label="Reset demo data" kind="ghost" icon="reset" @click="reset" />
    </div>
  </div>
</template>

<style scoped>
/* Fixed to the bottom, not the top: LxShell's own header is itself
   position: fixed; top: 0 with z-index: 8100 (confirmed via computed style — its
   own CSS drives that top offset from the design system's --space-0 var, which is
   reused everywhere for "zero" and isn't safe to override just to make room for
   this). A bottom banner needs no coordination with the shell's layout at all. */
.demo-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  background: var(--color-brand);
  color: var(--color-inverse, #fff);
  font-size: 0.85rem;
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 9000;
  box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.2);
}
.demo-banner-label a {
  color: inherit;
  text-decoration: underline;
  margin-left: 0.5rem;
}
.demo-banner-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.demo-banner-role-label {
  font-weight: 600;
}
</style>
