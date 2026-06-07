import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect } from "vitest";
import { ProspectTable } from "./ProspectTable";
import type { UIProspect } from "./constants";

function uiProspect(overrides: Partial<UIProspect> = {}): UIProspect {
  return {
    id: "p-1", initials: "AL", avatarColor: "bg-[#d8e3fb]", name: "Alice",
    company: "Acme", position: "CEO", email: "a@acme.com", phone: "", whatsapp: "",
    telegramUsername: "", sourceName: "", status: "Новый", consentStatus: "none",
    verifyStatus: "not_checked", verifyScore: 0, ...overrides,
  };
}

const baseProps = {
  loading: false, totalCount: 1, page: 1, totalPages: 1,
  rangeStart: 1, rangeEnd: 1, onPageChange: vi.fn(),
};

describe("ProspectTable consent toggle", () => {
  it("granting: a 'none' prospect toggles to obtained", async () => {
    const onToggle = vi.fn();
    render(<ProspectTable {...baseProps} prospects={[uiProspect({ consentStatus: "none" })]} onToggleConsent={onToggle} />);
    await userEvent.click(screen.getByTitle("Отметить согласие"));
    expect(onToggle).toHaveBeenCalledWith("p-1", "obtained");
  });

  it("withdrawing: an 'obtained' prospect toggles to withdrawn", async () => {
    const onToggle = vi.fn();
    render(<ProspectTable {...baseProps} prospects={[uiProspect({ consentStatus: "obtained" })]} onToggleConsent={onToggle} />);
    await userEvent.click(screen.getByTitle("Отозвать согласие"));
    expect(onToggle).toHaveBeenCalledWith("p-1", "withdrawn");
  });
});
