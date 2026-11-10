import { describe, it, expect, vi } from "vitest";

vi.mock("vue-i18n", () => ({ useI18n: () => ({ locale: { value: "en" } }) }));

const { default: useErrors } = await import("./errors.js");

describe("useErrors().get", () => {
  const { get } = useErrors();

  it("uses the server's message when there is one", () => {
    const err = { response: { status: 409, data: { error: "Bus is already booked", conflicts: [{ tripId: 1 }] } } };
    expect(get(err)).toEqual({ status: 409, message: "Bus is already booked", conflicts: [{ tripId: 1 }] });
  });

  it("falls back to a generic load error when the server answered without a message", () => {
    expect(get({ response: { status: 500, data: "" } }).message).toBe("errors.loadFailed");
  });

  it("says there is no connection when no response arrived (offline)", () => {
    expect(get({ message: "Network Error" })).toEqual({ status: 0, message: "errors.noConnection", conflicts: [] });
  });
});
