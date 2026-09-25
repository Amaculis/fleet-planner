import { computed } from "vue";
import { useI18n } from "vue-i18n";

// LxFilters' own default texts are hardcoded Latvian ("Filtri", "Atlasīt", "Notīrīt" —
// read from Filters-*.js in @dativa-lv/lx-ui), the same trap LxDataGrid had (see
// dataGridTexts.js), so without this the panel header and its Search/Clear buttons
// show Latvian regardless of the selected language. locales/*.json's "filterPanel"
// mirrors every key that component's fallback object defines.
export default function useFilterTexts() {
  const i18n = useI18n();
  return computed(() => i18n.tm("filterPanel"));
}
