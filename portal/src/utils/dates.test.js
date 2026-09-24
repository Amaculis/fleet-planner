import { describe, it, expect } from "vitest";
import {
  toIso,
  fromIso,
  addDays,
  addMonths,
  startOfMonth,
  startOfWeek,
  parseServerTimestamp,
  formatDateTime,
  formatTime,
} from "./dates";

describe("toIso", () => {
  it("formats a Date as YYYY-MM-DD", () => {
    expect(toIso(new Date(2026, 8, 24))).toBe("2026-09-24");
  });

  it("zero-pads single-digit month and day", () => {
    expect(toIso(new Date(2026, 0, 5))).toBe("2026-01-05");
  });
});

describe("fromIso", () => {
  it("parses back to the same local midnight Date toIso produced", () => {
    const date = fromIso("2026-09-24");
    expect(date.getFullYear()).toBe(2026);
    expect(date.getMonth()).toBe(8); // 0-indexed
    expect(date.getDate()).toBe(24);
    expect(date.getHours()).toBe(0);
  });

  it("round-trips through toIso", () => {
    expect(toIso(fromIso("2026-01-01"))).toBe("2026-01-01");
  });
});

describe("addDays", () => {
  it("adds a positive number of days", () => {
    expect(addDays("2026-09-24", 3)).toBe("2026-09-27");
  });

  it("subtracts with a negative number", () => {
    expect(addDays("2026-09-24", -3)).toBe("2026-09-21");
  });

  it("rolls over a month boundary", () => {
    expect(addDays("2026-09-29", 3)).toBe("2026-10-02");
  });

  it("rolls over a year boundary", () => {
    expect(addDays("2026-12-30", 3)).toBe("2027-01-02");
  });

  it("is a no-op for n=0", () => {
    expect(addDays("2026-09-24", 0)).toBe("2026-09-24");
  });
});

describe("addMonths", () => {
  it("adds a positive number of months", () => {
    expect(addMonths("2026-09-24", 2)).toBe("2026-11-01");
  });

  it("always clamps to the 1st of the resulting month, regardless of the input day", () => {
    expect(addMonths("2026-01-31", 1)).toBe("2026-02-01");
  });

  it("rolls over a year boundary", () => {
    expect(addMonths("2026-11-15", 2)).toBe("2027-01-01");
  });

  it("subtracts with a negative number", () => {
    expect(addMonths("2026-03-10", -1)).toBe("2026-02-01");
  });
});

describe("startOfMonth", () => {
  it("returns the 1st of the same month", () => {
    expect(startOfMonth("2026-09-24")).toBe("2026-09-01");
  });

  it("is idempotent on the 1st", () => {
    expect(startOfMonth("2026-09-01")).toBe("2026-09-01");
  });
});

describe("startOfWeek", () => {
  // Sep 2026: 21=Mon, 22=Tue, ..., 27=Sun — matches this app's Monday-first week
  // (createLx({ locale: { firstDayOfTheWeek: 1 } })).
  it("returns the same date when it's already Monday", () => {
    expect(startOfWeek("2026-09-21")).toBe("2026-09-21");
  });

  it("finds Monday for a mid-week date", () => {
    expect(startOfWeek("2026-09-24")).toBe("2026-09-21");
  });

  it("finds Monday for a Sunday (wraps back, not forward)", () => {
    expect(startOfWeek("2026-09-27")).toBe("2026-09-21");
  });

  it("crosses a month boundary correctly", () => {
    // Oct 1, 2026 is a Thursday; that week starts Mon Sep 28.
    expect(startOfWeek("2026-10-01")).toBe("2026-09-28");
  });
});

describe("parseServerTimestamp", () => {
  it("parses a full 'YYYY-MM-DDTHH:MM' timestamp", () => {
    const date = parseServerTimestamp("2026-09-24T14:30");
    expect(date.getFullYear()).toBe(2026);
    expect(date.getMonth()).toBe(8);
    expect(date.getDate()).toBe(24);
    expect(date.getHours()).toBe(14);
    expect(date.getMinutes()).toBe(30);
  });

  it("defaults the time to midnight when only a date is given", () => {
    const date = parseServerTimestamp("2026-09-24");
    expect(date.getHours()).toBe(0);
    expect(date.getMinutes()).toBe(0);
  });

  it("does not apply any timezone conversion — the wall-clock numbers pass straight through", () => {
    // Regression guard for the original Timeline bug this session found: treating
    // "T" as UTC or converting it would shift the hour depending on the machine's
    // local timezone offset, silently breaking every block's horizontal position.
    const date = parseServerTimestamp("2026-09-24T23:45");
    expect(date.getHours()).toBe(23);
    expect(date.getMinutes()).toBe(45);
  });
});

describe("formatDateTime / formatTime", () => {
  const date = new Date(2026, 8, 24, 14, 30);

  it("formatDateTime includes the year, a recognizable month, and the time", () => {
    const label = formatDateTime(date, "en-US");
    expect(label).toContain("2026");
    expect(label).toMatch(/Sep/i);
    expect(label).toMatch(/2:30/);
  });

  it("formatTime includes only the time, not the date", () => {
    const label = formatTime(date, "en-US");
    expect(label).not.toContain("2026");
    expect(label).toMatch(/2:30/);
  });
});
