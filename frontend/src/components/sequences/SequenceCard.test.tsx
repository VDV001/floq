import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SequenceCard } from "./SequenceCard";
import type { Sequence } from "@/lib/api";

vi.mock("@/components/ui/switch", () => ({
  // Forward aria-label + role so the two switches (active / approval) are
  // distinguishable by accessible name, mirroring the real Base UI Switch.
  Switch: ({ checked, onCheckedChange, ...props }: { checked: boolean; onCheckedChange: (v: boolean) => void; [k: string]: unknown }) => (
    <button role="switch" aria-checked={checked} aria-label={props["aria-label"] as string} onClick={() => onCheckedChange(!checked)}>
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
    require_approval: false,
    created_at: "2026-01-15T10:00:00Z",
    ...overrides,
  };
}

const ACTIVE_SWITCH = { name: "Активность секвенции" };
const APPROVAL_SWITCH = { name: "Требовать одобрение перед отправкой" };

const defaultProps = {
  isSelected: false,
  onSelect: vi.fn(),
  onToggle: vi.fn(),
  onApprovalToggle: vi.fn(),
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

  it("calls onToggle via the active switch", async () => {
    const onToggle = vi.fn();
    render(<SequenceCard sequence={makeSequence()} {...defaultProps} onToggle={onToggle} />);

    await userEvent.click(screen.getByRole("switch", ACTIVE_SWITCH));

    expect(onToggle).toHaveBeenCalledWith(false);
  });

  it("calls onApprovalToggle via the approval switch", async () => {
    const onApprovalToggle = vi.fn();
    render(
      <SequenceCard sequence={makeSequence({ require_approval: false })} {...defaultProps} onApprovalToggle={onApprovalToggle} />
    );

    await userEvent.click(screen.getByRole("switch", APPROVAL_SWITCH));

    expect(onApprovalToggle).toHaveBeenCalledWith(true);
  });

  it("reflects the approval gate state on its switch", () => {
    render(<SequenceCard sequence={makeSequence({ require_approval: true })} {...defaultProps} />);

    expect(screen.getByRole("switch", APPROVAL_SWITCH)).toHaveTextContent("on");
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
