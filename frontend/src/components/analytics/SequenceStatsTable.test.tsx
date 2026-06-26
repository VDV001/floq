import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { SequenceAnalyticsRow } from "@/lib/api";

import { SequenceStatsTable } from "./SequenceStatsTable";

function row(over: Partial<SequenceAnalyticsRow> = {}): SequenceAnalyticsRow {
  return {
    id: over.id ?? "seq-x",
    name: over.name ?? "Seq",
    sent: over.sent ?? 0,
    delivered: over.delivered ?? 0,
    opened: over.opened ?? 0,
    replied: over.replied ?? 0,
    converted: over.converted ?? 0,
    open_rate: over.open_rate ?? 0,
    reply_rate: over.reply_rate ?? 0,
    conversion_rate: over.conversion_rate ?? 0,
  };
}

function nameOrder(): string[] {
  return screen
    .getAllByRole("row")
    .slice(1) // skip header
    .map((r) => within(r).getAllByRole("cell")[0].textContent ?? "");
}

describe("SequenceStatsTable", () => {
  it("renders an empty state when there are no rows", () => {
    render(<SequenceStatsTable rows={[]} />);
    expect(screen.getByText(/Нет sequence.*активностью/i)).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("formats rates as percentages with one decimal", () => {
    render(<SequenceStatsTable rows={[row({ open_rate: 0.5, reply_rate: 0.123, conversion_rate: 0 })]} />);
    expect(screen.getByText("50.0%")).toBeInTheDocument();
    expect(screen.getByText("12.3%")).toBeInTheDocument();
    expect(screen.getByText("0.0%")).toBeInTheDocument();
  });

  it("sorts by sent descending by default", () => {
    render(
      <SequenceStatsTable
        rows={[
          row({ id: "a", name: "Low", sent: 1 }),
          row({ id: "b", name: "High", sent: 9 }),
          row({ id: "c", name: "Mid", sent: 5 }),
        ]}
      />,
    );
    expect(nameOrder()).toEqual(["High", "Mid", "Low"]);
  });

  it("toggles sort direction when the active column header is clicked", async () => {
    const user = userEvent.setup();
    render(
      <SequenceStatsTable
        rows={[
          row({ id: "a", name: "Low", sent: 1 }),
          row({ id: "b", name: "High", sent: 9 }),
        ]}
      />,
    );
    expect(nameOrder()).toEqual(["High", "Low"]);
    await user.click(screen.getByRole("button", { name: "Sent" }));
    expect(nameOrder()).toEqual(["Low", "High"]);
  });

  it("sorts by the string name column using localeCompare", async () => {
    const user = userEvent.setup();
    render(
      <SequenceStatsTable
        rows={[
          row({ id: "a", name: "alpha", sent: 1 }),
          row({ id: "b", name: "gamma", sent: 9 }),
          row({ id: "c", name: "beta", sent: 5 }),
        ]}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Sequence" }));
    // name, descending => reverse alphabetical
    expect(nameOrder()).toEqual(["gamma", "beta", "alpha"]);
  });

  it("switches to another numeric column resetting to descending", async () => {
    const user = userEvent.setup();
    render(
      <SequenceStatsTable
        rows={[
          row({ id: "a", name: "FewOpens", sent: 9, open_rate: 0.1 }),
          row({ id: "b", name: "ManyOpens", sent: 1, open_rate: 0.9 }),
        ]}
      />,
    );
    expect(nameOrder()).toEqual(["FewOpens", "ManyOpens"]);
    await user.click(screen.getByRole("button", { name: "Open %" }));
    expect(nameOrder()).toEqual(["ManyOpens", "FewOpens"]);
  });
});
