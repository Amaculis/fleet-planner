import { computed } from "vue";
import { useI18n } from "vue-i18n";

// LxStateDisplay dictionaries, also used by LxDataGrid `type: "state"` columns — one
// place for status colors instead of per-view CSS classes. displayType values are
// LxStateDisplay's own theme states (new/ongoing/finished/inactive/...).
const TRIP_STATUS_TYPES = {
  planned: "new",
  in_progress: "ongoing",
  completed: "finished",
  cancelled: "inactive",
};

const PAYMENT_STATUS_TYPES = {
  unpaid: "inactive",
  reserved: "new",
  advance_paid: "ongoing",
  paid: "finished",
};

const BUS_STATUS_TYPES = {
  active: "finished",
  maintenance: "ongoing",
  retired: "inactive",
};

export default function useStateDictionaries() {
  const i18n = useI18n();
  const toDictionary = (types, prefix) =>
    Object.entries(types).map(([value, displayType]) => ({
      value,
      displayName: i18n.t(`${prefix}.${value}`),
      displayType,
    }));

  return {
    tripStatusDictionary: computed(() => toDictionary(TRIP_STATUS_TYPES, "tripStatus")),
    paymentStatusDictionary: computed(() => toDictionary(PAYMENT_STATUS_TYPES, "paymentStatus")),
    busStatusDictionary: computed(() => toDictionary(BUS_STATUS_TYPES, "busStatus")),
    // Keyed by the row's activeState: "true" / "false", or "erased" for an anonymized
    // driver/client (LxStateDisplay compares values as strings).
    activeDictionary: computed(() => [
      { value: "true", displayName: i18n.t("common.active"), displayType: "finished" },
      { value: "false", displayName: i18n.t("common.inactive"), displayType: "inactive" },
      { value: "erased", displayName: i18n.t("common.erased"), displayType: "deleted" },
    ]),
  };
}
