import { createI18n } from "vue-i18n";
import en from "@/locales/en.json";
import lv from "@/locales/lv.json";
import ru from "@/locales/ru.json";
import { APP_CONFIG } from "@/constants";
import { initialLocale } from "@/i18n/locale";

export { initialLocale };

const locale = initialLocale();
if (typeof document !== "undefined") document.documentElement.lang = locale;

const i18n = createI18n({
  locale,
  fallbackLocale: APP_CONFIG.fallbackLocale,
  legacy: false,
  messages: { en, lv, ru },
});

export default i18n;

// Texts LX UI shows by itself, outside any component's `texts` prop: the error when a
// page can't be loaded (offline or a failed chunk). LX UI only knows Latvian, so these
// are passed to createLx (main.js) and again on every language switch (hooks/language.js).
export function lxComponentTexts() {
  const { t } = i18n.global;
  return {
    LxDynamicImport: { offline: t("lxDynamicImport.offline"), loadFailed: t("lxDynamicImport.loadFailed") },
  };
}
