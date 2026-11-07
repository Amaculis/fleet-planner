import { useI18n } from "vue-i18n";
import { LOCALE_STORAGE_KEY } from "@/constants";

// Switches vue-i18n's active locale. LxShell's language picker only emits which
// language was chosen (see layouts/MainLayout.vue); it does not know how to apply it.
export function useLanguageSwitcher() {
  const i18n = useI18n();

  function switchLocale(locale) {
    i18n.locale.value = locale;
    document.documentElement.lang = locale;
    try {
      localStorage.setItem(LOCALE_STORAGE_KEY, locale);
    } catch {
      // Private browsing / blocked storage: the choice just doesn't persist.
    }
  }

  return { switchLocale };
}
