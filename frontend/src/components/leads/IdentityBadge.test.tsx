import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { IdentityBadge } from "./IdentityBadge";
import type { IdentitySummary } from "@/lib/api";

const leadID = "11111111-1111-1111-1111-111111111111";
const otherLeadID = "22222222-2222-2222-2222-222222222222";

function fullIdentity(overrides: Partial<IdentitySummary> = {}): IdentitySummary {
  return {
    id: "id-1",
    email: "alice@acme.com",
    phone: "+79991234567",
    telegram_username: "alice_bot",
    linked_lead_ids: [leadID],
    ...overrides,
  };
}

describe("IdentityBadge", () => {
  beforeEach(() => {
    // jsdom does not provide a clipboard implementation by default —
    // a minimal stub lets the copy interactions exercise the
    // happy-path without crashing the test runner.
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
  });

  it("returns null when identity is missing", () => {
    const { container } = render(<IdentityBadge identity={undefined} currentLeadId={leadID} />);
    expect(container.firstChild).toBeNull();
  });

  it("returns null when identity has no canonical identifiers", () => {
    const { container } = render(
      <IdentityBadge
        identity={{ id: "id-1", linked_lead_ids: [leadID] }}
        currentLeadId={leadID}
      />
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders one pill per non-empty identifier", () => {
    render(<IdentityBadge identity={fullIdentity()} currentLeadId={leadID} />);
    expect(screen.getByText("alice@acme.com")).toBeInTheDocument();
    expect(screen.getByText("+79991234567")).toBeInTheDocument();
    expect(screen.getByText("@alice_bot")).toBeInTheDocument();
  });

  it("hides identifiers that are empty strings or missing", () => {
    render(
      <IdentityBadge
        identity={fullIdentity({ phone: undefined, telegram_username: "" })}
        currentLeadId={leadID}
      />
    );
    expect(screen.getByText("alice@acme.com")).toBeInTheDocument();
    expect(screen.queryByText("+79991234567")).not.toBeInTheDocument();
    expect(screen.queryByText("@alice_bot")).not.toBeInTheDocument();
  });

  it("copies the identifier on click and shows feedback", async () => {
    render(<IdentityBadge identity={fullIdentity()} currentLeadId={leadID} />);
    const emailPill = screen.getByLabelText(/Email alice@acme.com/);
    fireEvent.click(emailPill);
    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith("alice@acme.com");
    });
  });

  it("hides sibling counter when current lead is the only linked one", () => {
    render(
      <IdentityBadge identity={fullIdentity({ linked_lead_ids: [leadID] })} currentLeadId={leadID} />
    );
    expect(screen.queryByText(/связанных лида/)).not.toBeInTheDocument();
    expect(screen.queryByText(/\+1 связанный лид/)).not.toBeInTheDocument();
  });

  it("shows singular sibling counter for exactly one other lead", () => {
    render(
      <IdentityBadge
        identity={fullIdentity({ linked_lead_ids: [leadID, otherLeadID] })}
        currentLeadId={leadID}
      />
    );
    expect(screen.getByText(/\+1 связанный лид/)).toBeInTheDocument();
  });

  it("shows plural sibling counter for ≥2 other leads", () => {
    render(
      <IdentityBadge
        identity={fullIdentity({
          linked_lead_ids: [leadID, otherLeadID, "33333333-3333-3333-3333-333333333333"],
        })}
        currentLeadId={leadID}
      />
    );
    expect(screen.getByText(/\+2 связанных лида/)).toBeInTheDocument();
  });

  it("excludes the current lead from sibling count even if listed", () => {
    render(
      <IdentityBadge
        identity={fullIdentity({
          linked_lead_ids: [leadID, leadID, otherLeadID], // duplicate self should be skipped
        })}
        currentLeadId={leadID}
      />
    );
    expect(screen.getByText(/\+1 связанный лид/)).toBeInTheDocument();
  });

  it("uses semantic section landmark for accessibility", () => {
    render(<IdentityBadge identity={fullIdentity()} currentLeadId={leadID} />);
    expect(screen.getByLabelText("Связанные каналы контакта")).toBeInTheDocument();
  });
});
