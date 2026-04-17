import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { getTimeAgo, getInitials } from "./format";

describe("getTimeAgo", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-04-17T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns 'Только что' for less than 1 minute ago", () => {
    expect(getTimeAgo("2026-04-17T12:00:00Z")).toBe("Только что");
    expect(getTimeAgo("2026-04-17T11:59:01Z")).toBe("Только что");
  });

  it("returns 'Только что' for exactly now (0 diff)", () => {
    expect(getTimeAgo("2026-04-17T12:00:00Z")).toBe("Только что");
  });

  it("returns minutes for 1-59 minutes ago", () => {
    expect(getTimeAgo("2026-04-17T11:59:00Z")).toBe("1 мин");
    expect(getTimeAgo("2026-04-17T11:30:00Z")).toBe("30 мин");
    expect(getTimeAgo("2026-04-17T11:01:00Z")).toBe("59 мин");
  });

  it("returns hours for 1-23 hours ago", () => {
    expect(getTimeAgo("2026-04-17T11:00:00Z")).toBe("1 ч");
    expect(getTimeAgo("2026-04-17T00:00:00Z")).toBe("12 ч");
    expect(getTimeAgo("2026-04-16T13:00:00Z")).toBe("23 ч");
  });

  it("returns days for 24+ hours ago", () => {
    expect(getTimeAgo("2026-04-16T12:00:00Z")).toBe("1 д");
    expect(getTimeAgo("2026-04-10T12:00:00Z")).toBe("7 д");
    expect(getTimeAgo("2026-03-17T12:00:00Z")).toBe("31 д");
  });

  it("handles future dates as 'Только что' (negative diff floors to < 1 min)", () => {
    // A date slightly in the future results in negative diff, Math.floor gives negative mins => < 1
    expect(getTimeAgo("2026-04-17T12:05:00Z")).toBe("Только что");
  });

  it("handles boundary at exactly 1 minute", () => {
    expect(getTimeAgo("2026-04-17T11:59:00Z")).toBe("1 мин");
  });

  it("handles boundary at exactly 1 hour", () => {
    expect(getTimeAgo("2026-04-17T11:00:00Z")).toBe("1 ч");
  });

  it("handles boundary at exactly 24 hours", () => {
    expect(getTimeAgo("2026-04-16T12:00:00Z")).toBe("1 д");
  });
});

describe("getInitials", () => {
  it("returns first two initials for two-word name", () => {
    expect(getInitials("John Doe")).toBe("JD");
  });

  it("returns first two initials for three-word name", () => {
    expect(getInitials("John Michael Doe")).toBe("JM");
  });

  it("returns single initial for single-word name", () => {
    expect(getInitials("John")).toBe("J");
  });

  it("uppercases lowercase initials", () => {
    expect(getInitials("john doe")).toBe("JD");
  });

  it("handles cyrillic names", () => {
    expect(getInitials("Даниил Петров")).toBe("ДП");
  });

  it("limits to 2 characters max", () => {
    expect(getInitials("A B C D E")).toBe("AB");
  });

  it("handles single character name", () => {
    expect(getInitials("A")).toBe("A");
  });

  it("handles name with extra spaces between words", () => {
    // split(" ") on "  John  Doe" = ["", "", "John", "", "Doe"]
    // ""[0] = undefined; Array.join treats undefined as ""
    // so join("") = "JD", slice(0,2) = "JD"
    expect(getInitials("  John  Doe")).toBe("JD");
  });

  it("handles mixed case", () => {
    expect(getInitials("mARY jANE")).toBe("MJ");
  });
});
