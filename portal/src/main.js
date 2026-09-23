import { createApp } from "vue";
import { createPinia } from "pinia";
import {
  createLx,
  LxShell,
  LxButton,
  LxTextInput,
  LxErrorPage,
  LxAccessibilitySettings,
  LxLoaderView,
  LxInfoBox,
  LxDataGrid,
  LxValuePicker,
  LxDateTimePicker,
  LxToggle,
  LxBadge,
} from "@dativa-lv/lx-ui";

// Reset & fonts (always required).
import "@dativa-lv/lx-ui/dist/styles/lx-reset.css";
import "@dativa-lv/lx-ui/dist/styles/lx-fonts-carbon.css";
// Theme: one product theme + light/dark/contrast utility themes.
import "@dativa-lv/lx-ui/dist/styles/lx-pt-carbon.css";
import "@dativa-lv/lx-ui/dist/styles/lx-ut-carbon-light.css";
import "@dativa-lv/lx-ui/dist/styles/lx-ut-carbon-dark.css";
import "@dativa-lv/lx-ui/dist/styles/lx-ut-carbon-contrast.css";
// Components this app actually uses ("only import what you need" — but every
// component actually used, checked here by cross-referencing each component's real
// CSS class names, from its own source, against which .css file defines them; see
// the conversation history for the audit that found the gaps this list used to have).
import "@dativa-lv/lx-ui/dist/styles/lx-shell-grid.css"; // LxShell, LxWidget
import "@dativa-lv/lx-ui/dist/styles/lx-buttons.css"; // LxButton, LxErrorPage's actions
import "@dativa-lv/lx-ui/dist/styles/lx-inputs.css"; // LxTextInput, LxNumberInput, LxTextArea
import "@dativa-lv/lx-ui/dist/styles/lx-forms.css"; // LxSection, LxRow
import "@dativa-lv/lx-ui/dist/styles/lx-forms-grid.css";
import "@dativa-lv/lx-ui/dist/styles/lx-notifications.css"; // LxShell's alert/toast panel
import "@dativa-lv/lx-ui/dist/styles/lx-loaders.css"; // LxLoaderView
import "@dativa-lv/lx-ui/dist/styles/lx-data-grid.css"; // LxDataGrid
import "@dativa-lv/lx-ui/dist/styles/lx-value-pickers.css"; // LxValuePicker
import "@dativa-lv/lx-ui/dist/styles/lx-date-pickers.css"; // LxDateTimePicker
import "@dativa-lv/lx-ui/dist/styles/lx-toggles.css"; // LxToggle
import "@dativa-lv/lx-ui/dist/styles/lx-badges.css"; // LxBadge
import "@dativa-lv/lx-ui/dist/styles/lx-info-boxes.css"; // LxInfoBox — distinct from lx-notifications.css
import "@dativa-lv/lx-ui/dist/styles/lx-expanders.css"; // LxAccessibilitySettings
import "@dativa-lv/lx-ui/dist/styles/lx-stack.css"; // LxAccessibilitySettings' internal LxStack

import App from "@/App.vue";
import router from "@/router";
import events from "@/router/events";
import i18n from "@/i18n";
import { APP_CONFIG } from "@/constants";

const app = createApp(App);

app.use(createPinia());
events(router);
app.use(router);
app.use(i18n);

// No authUrl/authClientId: this app has no external OIDC provider (see
// stores/auth.js) — createLx is used only for its UI plumbing (icons, locale,
// first-day-of-week, component preloading, chunk base URL). Same option shape
// verified against source for the calendar island (see docs/lx-ui-integration.md
// and web/vue/main.js) — systemId/authXxx from the skill's example are dropped
// because they were never verified and aren't needed without their auth module.
app.use(createLx, {
  locale: { tag: APP_CONFIG.defaultLocale, firstDayOfTheWeek: 1 },
  preload: {
    components: [
      LxShell, LxButton, LxTextInput, LxErrorPage, LxAccessibilitySettings, LxLoaderView, LxInfoBox,
      LxDataGrid, LxValuePicker, LxDateTimePicker, LxToggle, LxBadge,
    ],
  },
  publicUrl: window.location.origin + APP_CONFIG.publicUrl,
  environment: APP_CONFIG.environment,
});

app.mount("#app");
