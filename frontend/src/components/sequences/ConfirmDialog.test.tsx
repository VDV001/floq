import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmDialog } from "./ConfirmDialog";

describe("ConfirmDialog", () => {
  it("renders title and message", () => {
    render(<ConfirmDialog title="Удалить шаг" message="Вы уверены?" onConfirm={vi.fn()} onCancel={vi.fn()} />);

    expect(screen.getByText("Удалить шаг")).toBeInTheDocument();
    expect(screen.getByText("Вы уверены?")).toBeInTheDocument();
  });

  it("calls onConfirm on confirm button click", async () => {
    const onConfirm = vi.fn();
    render(<ConfirmDialog title="T" message="M" onConfirm={onConfirm} onCancel={vi.fn()} />);

    await userEvent.click(screen.getByText("Удалить"));

    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("calls onCancel on cancel button click", async () => {
    const onCancel = vi.fn();
    render(<ConfirmDialog title="T" message="M" onConfirm={vi.fn()} onCancel={onCancel} />);

    await userEvent.click(screen.getByText("Отмена"));

    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});
