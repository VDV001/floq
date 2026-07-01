import { describe, it, expect } from "vitest";
import { launchBlockReason } from "./prospectEligibility";
import type { Prospect } from "./api";

function prospect(over: Partial<Prospect> = {}): Prospect {
  return {
    id: "p1",
    name: "Ivan",
    email: "ivan@acme.com",
    status: "new",
    verify_status: "valid",
    ...over,
  } as Prospect;
}

describe("launchBlockReason", () => {
  it("returns null for a verified, new prospect (launchable)", () => {
    expect(launchBlockReason(prospect({ verify_status: "valid" }))).toBeNull();
  });

  it("flags an invalid email", () => {
    expect(launchBlockReason(prospect({ verify_status: "invalid" }))).toMatch(/невалид/i);
  });

  it("flags an unverified email that is set — the #221 case (Ivan / 'mmvs')", () => {
    expect(
      launchBlockReason(prospect({ email: "mmvs", verify_status: "not_checked" })),
    ).toMatch(/не провер/i);
  });

  it("does not flag not_checked when there is no email", () => {
    expect(launchBlockReason(prospect({ email: "", verify_status: "not_checked" }))).toBeNull();
  });

  it("does not flag a risky email (backend still allows launch)", () => {
    expect(launchBlockReason(prospect({ verify_status: "risky" }))).toBeNull();
  });
});
