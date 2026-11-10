// Kept apart from i18n/index.js so offline.html can use it without vue-i18n.
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
