import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { OnecMappingRule } from "@/lib/api";
import { OnecMappingEditor } from "./OnecMappingEditor";

function rule(over: Partial<OnecMappingRule> = {}): OnecMappingRule {
  return {
    external_type: over.external_type ?? "Документ.Оплата",
    kind: over.kind ?? "payment",
    email_field: over.email_field ?? "email",
    name_field: over.name_field,
    company_field: over.company_field,
  };
}

function baseProps() {
  return {
    rules: [rule()],
    addRule: vi.fn(),
    updateRule: vi.fn(),
    removeRule: vi.fn(),
    saving: false,
    result: null,
    setResult: () => {},
    onSave: vi.fn(),
  };
}

describe("OnecMappingEditor", () => {
  it("renders a row per rule with the kind selected", () => {
    render(<OnecMappingEditor {...baseProps()} />);
    const rows = screen.getAllByRole("row").filter((r) => within(r).queryAllByRole("textbox").length > 0);
    expect(rows).toHaveLength(1);
    const kind = screen.getByRole("combobox") as HTMLSelectElement;
    expect(kind.value).toBe("payment");
  });

  it("calls addRule when adding", async () => {
    const props = baseProps();
    render(<OnecMappingEditor {...props} />);
    await userEvent.click(screen.getByRole("button", { name: /Добавить/i }));
    expect(props.addRule).toHaveBeenCalled();
  });

  it("calls removeRule with the row index", async () => {
    const props = { ...baseProps(), rules: [rule(), rule({ external_type: "Документ.Заказ", kind: "order_status" })] };
    render(<OnecMappingEditor {...props} />);
    const removeButtons = screen.getAllByRole("button", { name: /Удалить/i });
    await userEvent.click(removeButtons[1]);
    expect(props.removeRule).toHaveBeenCalledWith(1);
  });

  it("calls updateRule when the kind changes", async () => {
    const props = baseProps();
    render(<OnecMappingEditor {...props} />);
    await userEvent.selectOptions(screen.getByRole("combobox"), "order_status");
    expect(props.updateRule).toHaveBeenCalledWith(0, { kind: "order_status" });
  });

  it("calls onSave", async () => {
    const props = baseProps();
    render(<OnecMappingEditor {...props} />);
    await userEvent.click(screen.getByRole("button", { name: /Сохранить маппинг/i }));
    expect(props.onSave).toHaveBeenCalled();
  });

  it("shows an empty state when there are no rules", () => {
    render(<OnecMappingEditor {...baseProps()} rules={[]} />);
    expect(screen.getByText(/Нет правил/i)).toBeInTheDocument();
  });
});
