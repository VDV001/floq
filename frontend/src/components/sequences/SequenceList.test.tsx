import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SequenceList } from "./SequenceList";
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

const defaultHandlers = {
  onSelect: vi.fn(),
  onToggle: vi.fn(),
  onEdit: vi.fn(),
  onDelete: vi.fn(),
};

describe("SequenceList", () => {
  it("renders heading", () => {
    render(<SequenceList loading={false} sequences={[]} selectedSeqId={null} {...defaultHandlers} />);
    expect(screen.getByText("Ваши кампании")).toBeInTheDocument();
  });

  it("shows empty state when no sequences", () => {
    render(<SequenceList loading={false} sequences={[]} selectedSeqId={null} {...defaultHandlers} />);
    expect(screen.getByText("Нет секвенций")).toBeInTheDocument();
  });

  it("does not show empty state while loading", () => {
    render(<SequenceList loading={true} sequences={[]} selectedSeqId={null} {...defaultHandlers} />);
    expect(screen.queryByText("Нет секвенций")).not.toBeInTheDocument();
  });

  it("renders sequence cards", () => {
    const sequences = [
      makeSequence({ id: "s1", name: "Campaign A" }),
      makeSequence({ id: "s2", name: "Campaign B" }),
    ];
    render(<SequenceList loading={false} sequences={sequences} selectedSeqId="s1" {...defaultHandlers} />);

    expect(screen.getByText("Campaign A")).toBeInTheDocument();
    expect(screen.getByText("Campaign B")).toBeInTheDocument();
  });

  it("calls onSelect with correct id", async () => {
    const onSelect = vi.fn();
    const sequences = [makeSequence({ id: "s1", name: "Campaign A" })];

    render(<SequenceList loading={false} sequences={sequences} selectedSeqId={null} onSelect={onSelect} onToggle={vi.fn()} onEdit={vi.fn()} onDelete={vi.fn()} />);

    await userEvent.click(screen.getByText("Campaign A"));
    expect(onSelect).toHaveBeenCalledWith("s1");
  });
});
