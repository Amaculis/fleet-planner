import { computed } from "vue";
import { useI18n } from "vue-i18n";

// LxShell's own default texts are hardcoded Latvian too — same class of bug as
// LxDataGrid's (see dataGridTexts.js): its source ships one large literal object
// (Shell-pZCSOo8K.js) covering everything from the confirm-dialog Yes/No buttons to
// the entire Accessibility settings page's copy. Passing only a handful of keys (as
// this app first did) leaves the rest — confirmModalPrimaryDefaultLabel ("Yes"),
// every accessibilitySettings.* string — silently falling back to Latvian regardless
// of the selected language. locales/*.json's "shell" key mirrors that object key for
// key.
export default function useShellTexts() {
  const i18n = useI18n();
  return computed(() => i18n.tm("shell"));
}
