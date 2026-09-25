import { ref, computed } from "vue";

// Inline field validation for the edit forms. `rules` returns { field: message } for
// every failing field (an empty object when all is well); nothing is shown until the
// first validate() call — nobody wants a red "required" on a form they haven't
// touched — and after that the messages track the fields live, clearing as each one
// is fixed. The server re-checks everything regardless; this only exists so the form
// can say what's wrong next to the field instead of after a round trip.
export default function useFormValidation(rules) {
  const submitted = ref(false);
  const fieldErrors = computed(() => (submitted.value ? rules() : {}));

  // invalid + invalidationMessage travel together on every lx-ui input.
  const invalidProps = (field) => ({
    invalid: Boolean(fieldErrors.value[field]),
    invalidationMessage: fieldErrors.value[field] ?? "",
  });

  // Turns validation on and reports whether the form may be submitted.
  function validate() {
    submitted.value = true;
    return Object.keys(fieldErrors.value).length === 0;
  }

  return { fieldErrors, invalidProps, validate };
}
