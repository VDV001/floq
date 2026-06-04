import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

import { LeadsByStatusTable } from "./LeadsByStatusTable";

describe("LeadsByStatusTable", () => {
  beforeEach(() => pushMock.mockReset());

  it("renders status rows in funnel order with counts", () => {
    render(<LeadsByStatusTable byStatus={{ new: 10, qualified: 40, closed: 5 }} total={55} />);
    const rows = screen.getAllByRole("row").slice(1); // skip header
    // Funnel order: new before qualified before closed.
    expect(rows[0]).toHaveTextContent("Новые");
    expect(rows[0]).toHaveTextContent("10");
    expect(screen.getByText("Квалифицированные")).toBeInTheDocument();
  });

  it("navigates to the inbox on row click", async () => {
    const user = userEvent.setup();
    render(<LeadsByStatusTable byStatus={{ new: 10 }} total={10} />);
    await user.click(screen.getByText("Новые"));
    expect(pushMock).toHaveBeenCalledWith("/inbox");
  });

  it("renders an empty state when there are no leads", () => {
    render(<LeadsByStatusTable byStatus={{}} total={0} />);
    expect(screen.getByText(/Нет лидов/i)).toBeInTheDocument();
  });
});
