import { describe, it, expect } from "vitest";
import { mapProspects, mapConsentStatus, CONSENT_STYLES } from "./constants";

describe("consent mapping", () => {
  it("mapConsentStatus normalizes unknown to none", () => {
    expect(mapConsentStatus("obtained")).toBe("obtained");
    expect(mapConsentStatus("withdrawn")).toBe("withdrawn");
    expect(mapConsentStatus("none")).toBe("none");
    expect(mapConsentStatus("")).toBe("none");
    expect(mapConsentStatus("bogus")).toBe("none");
  });

  it("mapProspects carries id and consentStatus", () => {
    const ui = mapProspects([
      {
        id: "p-1", name: "Alice", company: "Acme", title: "CEO", email: "a@acme.com",
        phone: "", whatsapp: "", telegram_username: "", status: "new",
        consent_status: "obtained", verify_status: "not_checked", verify_score: 0,
      },
    ]);
    expect(ui[0].id).toBe("p-1");
    expect(ui[0].consentStatus).toBe("obtained");
  });

  it("mapProspects defaults missing consent to none", () => {
    const ui = mapProspects([
      {
        id: "p-2", name: "Bob", company: "", title: "", email: "b@b.com",
        phone: "", whatsapp: "", telegram_username: "", status: "new",
        verify_status: "not_checked", verify_score: 0,
      },
    ]);
    expect(ui[0].consentStatus).toBe("none");
  });

  it("CONSENT_STYLES has all three states", () => {
    expect(CONSENT_STYLES.none.label).toBeTruthy();
    expect(CONSENT_STYLES.obtained.label).toBeTruthy();
    expect(CONSENT_STYLES.withdrawn.label).toBeTruthy();
  });
});
