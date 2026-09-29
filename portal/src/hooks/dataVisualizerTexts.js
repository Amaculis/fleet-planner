import { computed } from "vue";
import { useI18n } from "vue-i18n";

// LxDataVisualizer's own default texts are hardcoded Latvian ("Grafiks", "Tabula",
// "no"/"līdz", ... — read from DataVisualizer-*.js in @dativa-lv/lx-ui), the same trap
// every other lx-ui component with built-in text has had (see dataGridTexts.js,
// filterTexts.js, formTexts.js): without this the graph/table switcher and axis
// captions show Latvian regardless of the selected language.
export default function useDataVisualizerTexts() {
  const i18n = useI18n();
  return computed(() => i18n.tm("dataVisualizer"));
}
