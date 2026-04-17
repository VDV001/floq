import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SequenceCard } from "./SequenceCard";
import type { Sequence } from "@/lib/api";

vi.mock("@/components/ui/switch", () => ({
  Switch: ({ checked, onCheckedChange }: { checked: boolean; onCheckedChange: (v: boolean) => void }) => (
    <button data-testid="switch" onClick={() => onCheckedChange(!checked)}>
      {checked ? "on" : "off"}
    </button>
  ),
}));

vi.mock("@/components/ui/separator", () => ({
  Separator: () => <hr />,
}));

function makeSequence(overrides: Partial<Sequence> = {}): Sequence {
  return {
    id: "seq-1",
    user_id: "u-1",
    name: "Cold Outreach",
    is_active: true,
    created_at: "2026-01-15T10:00:00Z",
    ...overrides,
  };
}

const defaultProps = {
  isSelected: false,
  onSelect: vi.fn(),
  onToggle: vi.fn(),
  onEdit: vi.fn(),
  onDelete: vi.fn(),
};

describe("SequenceCard", () => {
  it("renders sequence name and active badge", () => {
    render(<SequenceCard sequence={makeSequence()} {...defaultProps} />);

    expect(screen.getByText("Cold Outreach")).toBeInTheDocument();
    expect(screen.getAllByText("Активна").length).toBeGreaterThanOrEqual(1);
  });

  it("renders paused badge when inactive", () => {
    render(<SequenceCard sequence={makeSequence({ is_active: false })} {...defaultProps} />);

    expect(screen.getByText("Пауза")).toBeInTheDocument();
  });

  it("calls onSelect when card is clicked", async () => {
    const onSelect = vi.fn();
    render(<SequenceCard sequence={makeSequence()} {...defaultProps} onSelect={onSelect} />);

    await userEvent.click(screen.getByText("Cold Outreach"));

    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it("calls onToggle via switch", async () => {
    const onToggle = vi.fn();
    render(<SequenceCard sequence={makeSequence()} {...defaultProps} onToggle={onToggle} />);

    await userEvent.click(screen.getByTestId("switch"));

    expect(onToggle).toHaveBeenCalledWith(false);
  });

  it("calls onEdit without triggering onSelect", async () => {
    const onSelect = vi.fn();
    const onEdit = vi.fn();
    render(<SequenceCard sequence={makeSequence()} {...defaultProps} onSelect={onSelect} onEdit={onEdit} />);

    await userEvent.click(screen.getByText("Редактировать"));

    expect(onEdit).toHaveBeenCalledTimes(1);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("calls onDelete without triggering onSelect", async () => {
    const onSelect = vi.fn();
    const onDelete = vi.fn();
    render(<SequenceCard sequence={makeSequence()} {...defaultProps} onSelect={onSelect} onDelete={onDelete} />);

    await userEvent.click(screen.getByText("Удалить"));

    expect(onDelete).toHaveBeenCalledTimes(1);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("shows creation date", () => {
    render(<SequenceCard sequence={makeSequence()} {...defaultProps} />);

    expect(screen.getByText(/Создана/)).toBeInTheDocument();
  });
});
