<script setup>
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { LxShell } from "@dativa-lv/lx-ui";
import useAuthStore from "@/stores/auth";
import useAppStore from "@/stores/app";
import useNotifyStore from "@/stores/notify";
import useConfirmStore from "@/stores/confirm";
import { useLanguageSwitcher } from "@/hooks/language";
import useShellTexts from "@/hooks/shellTexts";

const route = useRoute();
const router = useRouter();
const i18n = useI18n();
const authStore = useAuthStore();
const appStore = useAppStore();
const notify = useNotifyStore();
const confirmStore = useConfirmStore();
const languageSwitcher = useLanguageSwitcher();

// Every page a signed-in user can reach. RBAC on each of these is enforced server-side
// (see internal/http/server.go's RequireRole groups) — this list only decides what a
// link shows, never what a request is allowed to do.
const role = computed(() => authStore.session?.role);
const isAdmin = computed(() => role.value === "admin");
const isPlanner = computed(() => role.value === "admin" || role.value === "dispatcher");
const isDriver = computed(() => role.value === "driver");

const nav = computed(() => {
  const items = [
    { id: "dashboard", label: i18n.t("pages.dashboard.title"), icon: "dashboard", to: { name: "dashboard" } },
  ];
  if (isPlanner.value) {
    items.push(
      // Icon names checked against the actual icon set (node_modules/@dativa-lv/
      // lx-ui/dist/Icon-CWjo-uMS.js's dynamic-import list) — "list" and "bus" don't
      // exist there and threw "Unknown variable dynamic import" at runtime. There is
      // no literal bus/vehicle icon in this set at all; driving-license is the
      // closest domain match.
      { id: "timeline", label: i18n.t("nav.timeline"), icon: "calendar", to: { name: "timeline" } },
      { id: "trips", label: i18n.t("nav.trips"), icon: "list-bulleted", to: { name: "trips" } },
      { id: "buses", label: i18n.t("nav.buses"), icon: "driving-license", to: { name: "buses" } },
      { id: "drivers", label: i18n.t("nav.drivers"), icon: "user-profile", to: { name: "drivers" } }
    );
  }
  if (isDriver.value) {
    items.push({ id: "myTrips", label: i18n.t("nav.myTrips"), icon: "list-bulleted", to: { name: "myTrips" } });
  }
  if (isAdmin.value) {
    items.push({ id: "users", label: i18n.t("nav.users"), icon: "settings", to: { name: "users" } });
  }
  items.push({ id: "accessibility", label: i18n.t("shellTexts.accessibility"), icon: "accessibility", to: { name: "accessibility" } });
  return items;
});

// Sub-routes (edit/new/detail) highlight their list page's nav entry.
const NAV_ALIASES = {
  busNew: "buses", busEdit: "buses",
  driverNew: "drivers", driverEdit: "drivers",
  tripNew: "trips", tripEdit: "trips", tripDetail: "trips",
  userNew: "users",
};
const selectedNavItems = computed(() => ({
  [NAV_ALIASES[route.name] ?? route.name]: true,
}));

const pageLabel = computed(() => {
  if (typeof route.meta.title === "function") return route.meta.title(i18n);
  return route.meta?.title ? i18n.t(route.meta.title) : "";
});

// LxShell's default (has-avatar unset) is a bare placeholder icon — no photo field
// exists on LxAvatar at all (checked its props directly): it only ever generates a
// deterministic color + icon or initials from a name, never displays an uploaded
// image. This app stores no first/last name for non-driver accounts (users only have
// an email — data minimisation, see CLAUDE.md's GDPR baseline), and LxShell's avatar
// needs both userInfo.firstName and userInfo.lastName truthy or it silently falls
// back to no initials at all (checked HeaderButtons' source) — so those two fields
// are synthesized from data the account actually has, not fabricated.
const userInfo = computed(() => {
  if (!authStore.isAuthorized) return null;
  const email = authStore.session.email;
  const localPart = email.split("@")[0] || email;
  return {
    description: email,
    firstName: localPart.charAt(0).toUpperCase() + localPart.slice(1),
    lastName: i18n.t(`roles.${authStore.session.role}`),
  };
});

const languages = computed(() =>
  ["en", "lv", "ru"].map((id) => ({ id, name: i18n.t(`languages.${id}`) }))
);
const selectedLanguage = computed({
  get: () => languages.value.find((l) => l.id === i18n.locale.value) ?? languages.value[0],
  set: (lang) => languageSwitcher.switchLocale(lang.id),
});

const shellTexts = useShellTexts();

function goBack(path) {
  path !== -1 ? router.push(path) : router.back();
}
function goHome() {
  router.push({ name: "dashboard" });
}
async function logout() {
  await authStore.logout();
  router.push({ name: "login" });
}
</script>

<template>
  <LxShell
    :route-name="route.name?.toString()"
    system-name="Bus Fleet"
    system-name-short="Fleet"
    :user-info="userInfo"
    has-avatar
    avatar-kind="initials"
    :nav-items="nav"
    :nav-items-selected="selectedNavItems"
    :page-label="pageLabel"
    :page-back-button-visible="false"
    :page-index-path="{ name: 'dashboard' }"
    :navigating="appStore.$state.isNavigating"
    :confirm-dialog-data="confirmStore"
    :texts="shellTexts"
    has-language-picker
    :languages="languages"
    v-model:selected-language="selectedLanguage"
    v-model:notifications="notify.notifications"
    @go-home="goHome"
    @go-back="goBack"
    @log-out="logout"
  >
    <router-view />
  </LxShell>
</template>
