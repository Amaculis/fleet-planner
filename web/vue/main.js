// Entry point for the lx-ui field islands bundle (web/static/js/calendar-island.js —
// filename kept from the first component added; it now covers every lx-ui field kind).
//
// This is deliberately the *only* JavaScript in the app that runs a framework. Every
// other page stays server-rendered templ + htmx. The pattern here — find mount points
// left by the server, hydrate exactly those elements, wire their output back into
// plain HTML form fields — is what lets a handful of Vue-powered widgets live inside an
// otherwise server-rendered app without turning the app into an SPA. See
// docs/lx-ui-integration.md for the full design and the reasoning behind it.
//
// Nothing here calls out to a network. lx-ui's own auth/systemId options (see
// createLx() below) are left unset on purpose: this app has its own session-cookie
// auth (internal/auth), and lx-ui's auth integration is for a different login model.
import { createApp } from "vue";
// No Pinia import here: lx-ui declares pinia as a plain (non-peer) dependency of its
// own, bundling and using its own internal Pinia instance. Their README's setup
// example (`app.use(createPinia())`) is for an app that also needs Pinia itself, not a
// requirement for createLx() to work. Installing and providing our own copy — as this
// file originally did — creates two different Pinia instances (ours, and lx-ui's own
// nested one) in the same page: a real bug, not a style choice, that surfaced as
// `TypeError: ei is not a function` inside createLx() at runtime, on every field kind.
// See docs/lx-ui-integration.md.
import {
  createLx,
  LxDateTimePicker,
  LxTextInput,
  LxToggle,
  LxValuePicker,
  LxToolbar,
  LxButton,
  LxInfoBox,
} from "@dativa-lv/lx-ui";
import CalendarField from "./CalendarIsland.vue";
import TextField from "./fields/TextField.vue";
import ToggleField from "./fields/ToggleField.vue";
import SelectField from "./fields/SelectField.vue";
import NavToolbar from "./chrome/NavToolbar.vue";
import ActionButton from "./chrome/ActionButton.vue";
import FlashBox from "./chrome/FlashBox.vue";

import "@dativa-lv/lx-ui/dist/bundles/lx-bt-demo.css";

// lx-ui splits *every* component — and, more aggressively, every icon — into its own
// tiny lazily-loaded chunk (their own docs/ComponentPreload.md describes this as a
// deliberate tradeoff for their large app shell, not a bug). Left unconfigured, this
// means a form page's first paint triggers a waterfall of individual chunk requests as
// each field mounts. `preload.components` is lx-ui's own documented fix: naming the
// components this app actually uses here front-loads their chunks together instead of
// one-by-one as each field happens to mount. It does not reach into every internal
// icon those components use — see docs/lx-ui-integration.md for what that leaves open.
const PRELOAD_COMPONENTS = [LxDateTimePicker, LxTextInput, LxToggle, LxValuePicker, LxToolbar, LxButton, LxInfoBox];

// lx-ui components read locale/date-format/week-start from here via useLx(), rather
// than from a prop on every component instance — the one global config call their docs
// ask for (docs/CreateLx.md).
function lxConfig(bcp47) {
  return {
    locale: {
      tag: bcp47,
      // Monday-first everywhere the app is used (EU); lx-ui's own default is Tuesday
      // per their docs, which reads as a documentation typo more than a real default,
      // so this is set explicitly rather than trusted.
      firstDayOfTheWeek: 1,
    },
    preload: { components: PRELOAD_COMPONENTS },
    // Best-evidenced fix for every lazy component chunk 404ing at /js/calendar-assets/
    // instead of /static/js/calendar-assets/ — NOT verified in a real browser (no
    // headless browser available here), so treat this as a strong hypothesis, not a
    // confirmed fix, until checked. Reasoning: the chunk-loading code that computes
    // these URLs lives inside lx-ui's pre-compiled dist bundle (this project's own
    // Vite `base` config has no effect on it), and does `new URL(chunkPath,
    // window.location.origin)` whenever publicUrl is unset — exactly reproducing what
    // was observed. `publicUrl` ("Public base URL of the application" per
    // docs/CreateLx.md) reads as the intended override; this project just never set
    // it. If chunks still 404 after this, the real mechanism is something else and
    // this guess was wrong — see docs/lx-ui-integration.md.
    publicUrl: window.location.origin + "/static",
  };
}

// Every field kind this app enhances. Each entry says which component renders it and
// how to build that component's props from the mount element's data-* attributes.
const FIELD_KINDS = {
  calendar: {
    component: CalendarField,
    props: (el) => ({
      initialValue: el.dataset.value || null,
      minValue: el.dataset.min || null,
      maxValue: el.dataset.max || null,
      labelledBy: el.dataset.labelledby || null,
    }),
    // The calendar's job is navigation, not data entry: picking a day jumps the page
    // there via ?date=, the same URL the plain prev/today/next links already use.
    onChange: (value) => {
      const url = new URL(window.location.href);
      url.searchParams.set("date", value);
      window.location.assign(url.toString());
    },
  },
  text: {
    component: TextField,
    props: (el, fallback) => ({
      initialValue: fallback?.value ?? "",
      placeholder: el.dataset.placeholder || null,
      required: el.dataset.required === "true",
      maxlength: el.dataset.maxlength || null,
      uppercase: el.dataset.uppercase === "true",
      labelId: el.dataset.labelledby || null,
    }),
    onChange: (value, fallback) => {
      if (fallback) fallback.value = value;
    },
  },
  toggle: {
    component: ToggleField,
    props: (el, fallback) => ({
      initialChecked: !!fallback?.checked,
      label: el.dataset.label || null,
      labelId: el.dataset.labelledby || null,
    }),
    onChange: (checked, fallback) => {
      if (fallback) fallback.checked = checked;
    },
  },
  select: {
    component: SelectField,
    props: (el, fallback) => ({
      items: JSON.parse(el.dataset.items || "[]"),
      initialValue: fallback?.value || null,
      nullable: el.dataset.nullable === "true",
      required: el.dataset.required === "true",
      labelId: el.dataset.labelledby || null,
    }),
    onChange: (value, fallback) => {
      if (fallback) fallback.value = value == null ? "" : String(value);
    },
  },
  // The three page-chrome kinds below have no single native "value": their fallback
  // (data-input-id) is the id of the whole plain element they replace — a <nav>, a
  // button, a flash banner — hidden on a successful mount the same way a plain
  // <input> would be. None of them use onChange; each handles its own interaction
  // internally (NavToolbar submits the logout form or navigates directly, ActionButton
  // clicks its own fallback button, FlashBox has no interaction at all).
  toolbar: {
    component: NavToolbar,
    props: (el) => ({
      items: JSON.parse(el.dataset.items || "[]"),
      logoutFormId: el.dataset.logoutFormId,
    }),
    onChange: () => {},
  },
  button: {
    component: ActionButton,
    props: (el, fallback) => ({
      label: el.dataset.label || "",
      kind: el.dataset.buttonKind || "primary",
      destructive: el.dataset.destructive === "true",
      fallbackId: fallback?.id,
    }),
    onChange: () => {},
  },
  notification: {
    component: FlashBox,
    props: (el) => ({
      variant: el.dataset.variant || "info",
      label: el.dataset.label || "",
    }),
    onChange: () => {},
  },
};

function mountFieldIslands() {
  document.querySelectorAll("[data-lx-field]").forEach((mount) => {
    const kind = FIELD_KINDS[mount.dataset.kind];
    if (!kind) {
      console.error(`calendar island: unknown field kind "${mount.dataset.kind}"`);
      return;
    }

    // The real, working control this island replaces. Left alone — not removed — so
    // a failure below leaves the page exactly as functional as it was without Vue.
    const fallback = mount.dataset.inputId ? document.getElementById(mount.dataset.inputId) : null;

    try {
      const app = createApp(kind.component, {
        ...kind.props(mount, fallback),
        onChange: (value) => kind.onChange(value, fallback),
      });
      // createLx is a plain Vue plugin object ({ install }), not a factory function —
      // confirmed against its actual source (src/lib.js: `export const createLx =
      // plugin`), not the docs' example code, which shows it called as `createLx()`.
      // Calling it as a function, as this line originally did, throws "createLx is
      // not a function" (minified to an opaque name at runtime) — Vue's plugin
      // signature is app.use(plugin, ...options), not app.use(plugin(options)).
      app.use(createLx, lxConfig(mount.dataset.locale || "en-US"));
      app.mount(mount);

      if (fallback) fallback.hidden = true;
      mount.hidden = false;
    } catch (err) {
      // The island never gets to take down the page: on any failure, the native
      // field (never hidden in the first place if we got here) is what the user
      // fills in instead. Errors still surface in the console for us.
      console.error(`calendar island: "${mount.dataset.kind}" field failed to mount; using the plain field`, err);
    }
  });
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", mountFieldIslands);
} else {
  mountFieldIslands();
}
