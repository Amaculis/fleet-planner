import { createI18n } from "vue-i18n";
import en from "@/locales/en.json";
import lv from "@/locales/lv.json";
import ru from "@/locales/ru.json";
import { APP_CONFIG } from "@/constants";

const i18n = createI18n({
  locale: APP_CONFIG.defaultLocale,
  fallbackLocale: APP_CONFIG.fallbackLocale,
  legacy: false,
  messages: { en, lv, ru },
});

export default i18n;
