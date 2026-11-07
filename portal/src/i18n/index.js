import { createI18n } from "vue-i18n";
import en from "@/locales/en.json";
import lv from "@/locales/lv.json";
import ru from "@/locales/ru.json";
import { APP_CONFIG, LOCALE_STORAGE_KEY } from "@/constants";

// The language picked last time (hooks/language.js saves it), else the browser's
// language if we support it, else the default.
export function initialLocale() {
  let saved = null;
  try {
    saved = localStorage.getItem(LOCALE_STORAGE_KEY);
  } catch {
    // Private browsing / blocked storage: fall through to the browser language.
  }
  if (APP_CONFIG.supportedLocales.includes(saved)) return saved;
  const browser = (navigator.language || "").slice(0, 2).toLowerCase();
  if (APP_CONFIG.supportedLocales.includes(browser)) return browser;
  return APP_CONFIG.defaultLocale;
}

const locale = initialLocale();
if (typeof document !== "undefined") document.documentElement.lang = locale;

const i18n = createI18n({
  locale,
  fallbackLocale: APP_CONFIG.fallbackLocale,
  legacy: false,
  messages: { en, lv, ru },
});

export default i18n;
