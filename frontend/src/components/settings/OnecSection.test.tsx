import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { OnecSection } from "./OnecSection";

function noop() {}

function baseProps() {
  return {
    baseURL: "https://1c.example.com",
    setBaseURL: vi.fn(),
    authType: "basic" as const,
    setAuthType: vi.fn(),
    authSecret: "",
    setAuthSecret: vi.fn(),
    maskedSecret: "...e123",
    isActive: false,
    setIsActive: vi.fn(),
    maskedWebhook: "...8de2",
    fullWebhook: null as string | null,
    regenerating: false,
    onRegenerate: vi.fn(),
    saving: false,
    saveResult: null,
    onSave: vi.fn(),
    testing: false,
    testResult: null,
    setTestResult: noop,
    onTest: vi.fn(),
  };
}

describe("OnecSection", () => {
  it("shows the masked secret as placeholder, never as a value", () => {
    render(<OnecSection {...baseProps()} />);
    const secret = screen.getByLabelText(/секрет/i) as HTMLInputElement;
    expect(secret.value).toBe("");
    expect(secret.placeholder).toContain("...e123");
  });

  it("calls onSave when the save button is clicked", async () => {
    const props = baseProps();
    render(<OnecSection {...props} />);
    await userEvent.click(screen.getByRole("button", { name: /Сохранить/i }));
    expect(props.onSave).toHaveBeenCalled();
  });

  it("calls onTest and renders the result banner", () => {
    const props = { ...baseProps(), testResult: { success: false, error: "Не удалось подключиться" } };
    render(<OnecSection {...props} />);
    expect(screen.getByText("Не удалось подключиться")).toBeInTheDocument();
  });

  it("reveals the full webhook secret once after regeneration", () => {
    const full = "a".repeat(64);
    render(<OnecSection {...baseProps()} fullWebhook={full} />);
    expect(screen.getByText(full)).toBeInTheDocument();
  });

  it("calls onRegenerate when the regenerate button is clicked", async () => {
    const props = baseProps();
    render(<OnecSection {...props} />);
    await userEvent.click(screen.getByRole("button", { name: /Сгенерировать/i }));
    expect(props.onRegenerate).toHaveBeenCalled();
  });

  it("toggles the active switch", async () => {
    const props = baseProps();
    render(<OnecSection {...props} />);
    await userEvent.click(screen.getByRole("checkbox", { name: /Включить/i }));
    expect(props.setIsActive).toHaveBeenCalledWith(true);
  });
});
