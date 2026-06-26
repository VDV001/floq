import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { AIDraftPanel } from "./AIDraftPanel";

describe("AIDraftPanel", () => {
  it("splits the draft body into one paragraph per double newline", () => {
    render(
      <AIDraftPanel
        draftBody={"Первый абзац\n\nВторой абзац"}
        onEdit={vi.fn()}
        onSend={vi.fn()}
      />,
    );
    expect(screen.getByText("Первый абзац")).toBeInTheDocument();
    expect(screen.getByText("Второй абзац")).toBeInTheDocument();
  });

  it("renders a single paragraph when there is no blank-line separator", () => {
    render(<AIDraftPanel draftBody={"Одна строка"} onEdit={vi.fn()} onSend={vi.fn()} />);
    expect(screen.getByText("Одна строка")).toBeInTheDocument();
  });

  it("invokes onEdit when the edit button is clicked", () => {
    const onEdit = vi.fn();
    render(<AIDraftPanel draftBody="x" onEdit={onEdit} onSend={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /Редактировать/ }));
    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it("invokes onSend when the send button is clicked", () => {
    const onSend = vi.fn();
    render(<AIDraftPanel draftBody="x" onEdit={vi.fn()} onSend={onSend} />);
    fireEvent.click(screen.getByRole("button", { name: /Отправить/ }));
    expect(onSend).toHaveBeenCalledTimes(1);
  });

  it("shows the smart-draft badge and automation toggles", () => {
    render(<AIDraftPanel draftBody="x" onEdit={vi.fn()} onSend={vi.fn()} />);
    expect(screen.getByText("Умный черновик")).toBeInTheDocument();
    expect(screen.getByText("Авто-фоллоуапы")).toBeInTheDocument();
    expect(screen.getByText("Согласование черновиков")).toBeInTheDocument();
  });
});
