import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AddStepForm } from "./AddStepForm";

describe("AddStepForm", () => {
  it("renders channel buttons and inputs", () => {
    render(<AddStepForm onAdd={vi.fn()} onCancel={vi.fn()} />);

    expect(screen.getByText("Email")).toBeInTheDocument();
    expect(screen.getByText("Telegram")).toBeInTheDocument();
    expect(screen.getByText("Звонок")).toBeInTheDocument();
    expect(screen.getByText("Добавить")).toBeInTheDocument();
    expect(screen.getByText("Отмена")).toBeInTheDocument();
  });

  it("calls onAdd with default values", async () => {
    const onAdd = vi.fn().mockResolvedValue(undefined);
    render(<AddStepForm onAdd={onAdd} onCancel={vi.fn()} />);

    await userEvent.click(screen.getByText("Добавить"));

    expect(onAdd).toHaveBeenCalledWith({
      channel: "email",
      delay_days: 0,
      prompt_hint: "первое касание",
    });
  });

  it("switches channel to telegram", async () => {
    const onAdd = vi.fn().mockResolvedValue(undefined);
    render(<AddStepForm onAdd={onAdd} onCancel={vi.fn()} />);

    await userEvent.click(screen.getByText("Telegram"));
    await userEvent.click(screen.getByText("Добавить"));

    expect(onAdd).toHaveBeenCalledWith(
      expect.objectContaining({ channel: "telegram" })
    );
  });

  it("updates delay value", async () => {
    const onAdd = vi.fn().mockResolvedValue(undefined);
    render(<AddStepForm onAdd={onAdd} onCancel={vi.fn()} />);

    const delayInput = screen.getByDisplayValue("0");
    await userEvent.clear(delayInput);
    await userEvent.type(delayInput, "3");
    await userEvent.click(screen.getByText("Добавить"));

    expect(onAdd).toHaveBeenCalledWith(
      expect.objectContaining({ delay_days: 3 })
    );
  });

  it("updates hint value", async () => {
    const onAdd = vi.fn().mockResolvedValue(undefined);
    render(<AddStepForm onAdd={onAdd} onCancel={vi.fn()} />);

    const hintInput = screen.getByDisplayValue("первое касание");
    await userEvent.clear(hintInput);
    await userEvent.type(hintInput, "follow up");
    await userEvent.click(screen.getByText("Добавить"));

    expect(onAdd).toHaveBeenCalledWith(
      expect.objectContaining({ prompt_hint: "follow up" })
    );
  });

  it("calls onCancel", async () => {
    const onCancel = vi.fn();
    render(<AddStepForm onAdd={vi.fn()} onCancel={onCancel} />);

    await userEvent.click(screen.getByText("Отмена"));

    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("uses default hint when hint is cleared", async () => {
    const onAdd = vi.fn().mockResolvedValue(undefined);
    render(<AddStepForm onAdd={onAdd} onCancel={vi.fn()} />);

    const hintInput = screen.getByDisplayValue("первое касание");
    await userEvent.clear(hintInput);
    await userEvent.click(screen.getByText("Добавить"));

    expect(onAdd).toHaveBeenCalledWith(
      expect.objectContaining({ prompt_hint: "первое касание" })
    );
  });
});
