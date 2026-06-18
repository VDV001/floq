import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { CostBreakdownRow } from "@/lib/api";

import { CostBreakdownTable } from "./CostBreakdownTable";

function row(over: Partial<CostBreakdownRow> = {}): CostBreakdownRow {
  return {
    request_type: over.request_type,
    model: over.model,
    calls: over.calls ?? 0,
    usd: over.usd ?? 0,
    tokens_in: over.tokens_in ?? 0,
    tokens_out: over.tokens_out ?? 0,
  };
}

function labelOrder(): string[] {
  // First cell of each body row is the label column.
  return screen
    .getAllByRole("row")
    .slice(1) // skip header
    .map((r) => within(r).getAllByRole("cell")[0].textContent ?? "");
}

describe("CostBreakdownTable", () => {
  it("renders an empty state when there are no rows", () => {
    render(<CostBreakdownTable title="Затраты" labelHeader="Тип" rows={[]} labelKey="request_type" />);
    expect(screen.getByText(/нет данных за период/i)).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("renders rows with the title and headers", () => {
    render(
      <CostBreakdownTable
        title="Затраты по типам"
        labelHeader="Тип запроса"
        rows={[row({ request_type: "qualify", calls: 3, usd: 1.5 })]}
        labelKey="request_type"
      />,
    );
    expect(screen.getByText("Затраты по типам")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Тип запроса" })).toBeInTheDocument();
    expect(screen.getByText("qualify")).toBeInTheDocument();
  });

  it("formats USD with 2 decimals at or above a cent and 4 decimals below", () => {
    render(
      <CostBreakdownTable
        title="t"
        labelHeader="Тип"
        rows={[
          row({ request_type: "big", usd: 1.5 }),
          row({ request_type: "cent", usd: 0.01 }),
          row({ request_type: "tiny", usd: 0.0001 }),
        ]}
        labelKey="request_type"
      />,
    );
    expect(screen.getByText("$1.50")).toBeInTheDocument();
    // boundary: 0.01 is not < 0.01, so 2 decimals
    expect(screen.getByText("$0.01")).toBeInTheDocument();
    expect(screen.getByText("$0.0001")).toBeInTheDocument();
  });

  it("sorts by usd descending by default", () => {
    render(
      <CostBreakdownTable
        title="t"
        labelHeader="Тип"
        rows={[
          row({ request_type: "low", usd: 1 }),
          row({ request_type: "high", usd: 9 }),
          row({ request_type: "mid", usd: 5 }),
        ]}
        labelKey="request_type"
      />,
    );
    expect(labelOrder()).toEqual(["high", "mid", "low"]);
  });

  it("toggles sort direction when the active column header is clicked", async () => {
    const user = userEvent.setup();
    render(
      <CostBreakdownTable
        title="t"
        labelHeader="Тип"
        rows={[
          row({ request_type: "low", usd: 1 }),
          row({ request_type: "high", usd: 9 }),
        ]}
        labelKey="request_type"
      />,
    );
    expect(labelOrder()).toEqual(["high", "low"]);
    await user.click(screen.getByRole("button", { name: "USD" }));
    expect(labelOrder()).toEqual(["low", "high"]);
  });

  it("switches sort key to the label column and sorts descending first", async () => {
    const user = userEvent.setup();
    render(
      <CostBreakdownTable
        title="t"
        labelHeader="Тип"
        rows={[
          row({ request_type: "alpha", usd: 1 }),
          row({ request_type: "gamma", usd: 9 }),
          row({ request_type: "beta", usd: 5 }),
        ]}
        labelKey="request_type"
      />,
    );
    await user.click(screen.getByRole("button", { name: "Тип" }));
    // localeCompare, descending => reverse alphabetical
    expect(labelOrder()).toEqual(["gamma", "beta", "alpha"]);
  });

  it("switches between two numeric columns resetting to descending", async () => {
    const user = userEvent.setup();
    render(
      <CostBreakdownTable
        title="t"
        labelHeader="Тип"
        rows={[
          row({ request_type: "few", calls: 1, usd: 9 }),
          row({ request_type: "many", calls: 8, usd: 1 }),
        ]}
        labelKey="request_type"
      />,
    );
    // default usd desc => few, many
    expect(labelOrder()).toEqual(["few", "many"]);
    await user.click(screen.getByRole("button", { name: "Вызовов" }));
    // calls desc => many, few
    expect(labelOrder()).toEqual(["many", "few"]);
  });

  it("falls back to a dash for rows missing the label key", () => {
    render(
      <CostBreakdownTable
        title="t"
        labelHeader="Модель"
        rows={[row({ model: undefined, usd: 1 })]}
        labelKey="model"
      />,
    );
    expect(labelOrder()).toEqual(["—"]);
  });

  it("backs the label column with the model field when labelKey is model", () => {
    render(
      <CostBreakdownTable
        title="t"
        labelHeader="Модель"
        rows={[row({ model: "gpt-4o", usd: 1 })]}
        labelKey="model"
      />,
    );
    expect(screen.getByText("gpt-4o")).toBeInTheDocument();
  });
});
