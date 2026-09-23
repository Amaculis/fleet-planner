import { computed } from "vue";
import { useI18n } from "vue-i18n";

// LxDataGrid's own default texts are hardcoded Latvian (checked its source directly:
// DataGrid-5n0g21CQ.js ships a literal { items: { singular: "ieraksts", ... }, ... }
// object as its fallback), so without this the row-count footer and every other
// built-in label shows Latvian regardless of the app's selected language. locales/*
// .json's "dataGrid" key mirrors every key that object defines.
export default function useDataGridTexts() {
  const i18n = useI18n();
  return computed(() => i18n.tm("dataGrid"));
}
