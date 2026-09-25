import { computed } from "vue";
import { useI18n } from "vue-i18n";

// The Save / Cancel footer every edit form shares, as LxForm actionDefinitions.
//
// Cancel is "secondary", not "ghost": LxForm sorts its footer actions by kind
// (primary / secondary / tertiary / additional) and silently drops any other value, so
// a ghost Cancel simply never rendered (found on the trip form). `saving`, `disabled`
// and the two handlers are read lazily, so plain refs/functions from the caller work.
export default function useFormActions({ saving, disabled, onSave, onCancel }) {
  const i18n = useI18n();

  const actionDefinitions = computed(() => [
    {
      id: "save",
      name: i18n.t("actions.save"),
      kind: "primary",
      icon: "save",
      loading: saving.value,
      disabled: disabled ? disabled.value : false,
    },
    { id: "cancel", name: i18n.t("actions.cancel"), kind: "secondary" },
  ]);

  function onAction(actionId) {
    if (actionId === "save") onSave();
    else if (actionId === "cancel") onCancel();
  }

  return { actionDefinitions, onAction };
}
