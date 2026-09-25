import { computed } from "vue";
import { useI18n } from "vue-i18n";

// LxForm's own default texts are hardcoded Latvian ("(obligāts)", "(neobligāts)",
// "Citas darbības", … — read from Form-*.js in @dativa-lv/lx-ui), the same trap
// LxDataGrid and LxFilters had (see dataGridTexts.js / filterTexts.js): without this
// a required field's marker and the form's overflow menu show Latvian regardless of
// the selected language. locales/*.json's "lxForm" mirrors every key its fallback
// object defines.
export default function useFormTexts() {
  const i18n = useI18n();
  return computed(() => i18n.tm("lxForm"));
}
