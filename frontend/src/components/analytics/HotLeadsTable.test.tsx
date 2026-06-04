import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { HotLead } from "@/lib/api";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

import { HotLeadsTable } from "./HotLeadsTable";

function lead(over: Partial<HotLead> = {}): HotLead {
  return {
    id: over.id ?? "lead-x",
    contact_name: over.contact_name ?? "Acme",
    channel: over.channel ?? "telegram",
    status: over.status ?? "qualified",
    score: over.score === undefined ? 50 : over.score,
    score_reason: over.score_reason ?? "",
    last_activity_at: over.last_activity_at ?? "2026-05-19T10:00:00Z",
    qualified_at: over.qualified_at ?? null,
  };
}

function rowOrder(): string[] {
  // First cell of each body row is the contact name.
  return screen
    .getAllByRole("row")
    .slice(1) // skip header
    .map((r) => within(r).getAllByRole("cell")[0].textContent ?? "");
}

describe("HotLeadsTable", () => {
  beforeEach(() => {
    pushMock.mockReset();
  });

  it("renders an empty state when there are no leads", () => {
    render(<HotLeadsTable leads={[]} />);
    expect(screen.getByText(/Нет лидов/i)).toBeInTheDocument();
  });

  it("sorts by score descending by default with null scores last", () => {
    render(
      <HotLeadsTable
        leads={[
          lead({ id: "a", contact_name: "Mid", score: 50 }),
          lead({ id: "b", contact_name: "Top", score: 90 }),
          lead({ id: "c", contact_name: "NoScore", score: null }),
        ]}
      />,
    );
    expect(rowOrder()).toEqual(["Top", "Mid", "NoScore"]);
  });

  it("toggles to ascending on a second score header click, nulls still last", async () => {
    const user = userEvent.setup();
    render(
      <HotLeadsTable
        leads={[
          lead({ id: "a", contact_name: "Mid", score: 50 }),
          lead({ id: "b", contact_name: "Top", score: 90 }),
          lead({ id: "c", contact_name: "NoScore", score: null }),
        ]}
      />,
    );
    await user.click(screen.getByRole("button", { name: /Скор/ }));
    // ascending: lowest score first, but NULL always sorts last.
    expect(rowOrder()).toEqual(["Mid", "Top", "NoScore"]);
  });

  it("navigates to the lead inbox on row click", async () => {
    const user = userEvent.setup();
    render(<HotLeadsTable leads={[lead({ id: "lead-42", contact_name: "Click Me" })]} />);
    await user.click(screen.getByText("Click Me"));
    expect(pushMock).toHaveBeenCalledWith("/inbox/lead-42");
  });
});
