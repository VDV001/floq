import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect } from "vitest";
import { ProspectSelector } from "./ProspectSelector";
import type { Prospect } from "@/lib/api";

function prospect(over: Partial<Prospect> = {}): Prospect {
  return {
    id: "p1",
    name: "Alice",
    company: "Acme",
    email: "a@acme.com",
    status: "new",
    ...over,
  } as Prospect;
}

const cbs = () => ({
  onToggle: vi.fn(),
  onSelectAll: vi.fn(),
  onLaunch: vi.fn(),
  onLaunchAllNew: vi.fn(),
});

describe("ProspectSelector", () => {
  it("shows the empty state with no prospects", () => {
    render(
      <ProspectSelector
        prospects={[]}
        selectedProspects={new Set()}
        selectedSeqId="seq1"
        launching={false}
        newProspectsCount={0}
        {...cbs()}
      />,
    );
    expect(screen.getByText(/Нет проспектов/)).toBeInTheDocument();
    expect(screen.getByText("Проспекты (0)")).toBeInTheDocument();
  });

  it("renders prospects with known and fallback status labels", () => {
    render(
      <ProspectSelector
        prospects={[
          prospect({ id: "a", name: "Alice", status: "converted" }),
          prospect({ id: "b", name: "Bob", status: "weird_status" as Prospect["status"], company: "", email: "bob@x.io" }),
        ]}
        selectedProspects={new Set()}
        selectedSeqId="seq1"
        launching={false}
        newProspectsCount={0}
        {...cbs()}
      />,
    );
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("лид")).toBeInTheDocument(); // converted → "лид"
    expect(screen.getByText("weird_status")).toBeInTheDocument(); // fallback = raw status
    expect(screen.getByText("bob@x.io")).toBeInTheDocument(); // falls back to email when no company
  });

  it("toggles select-all label and fires the callback", async () => {
    const c = cbs();
    const { rerender } = render(
      <ProspectSelector
        prospects={[prospect({ id: "a" }), prospect({ id: "b" })]}
        selectedProspects={new Set()}
        selectedSeqId="seq1"
        launching={false}
        newProspectsCount={0}
        {...c}
      />,
    );
    const btn = screen.getByText("Выбрать все");
    await userEvent.click(btn);
    expect(c.onSelectAll).toHaveBeenCalled();

    rerender(
      <ProspectSelector
        prospects={[prospect({ id: "a" }), prospect({ id: "b" })]}
        selectedProspects={new Set(["a", "b"])}
        selectedSeqId="seq1"
        launching={false}
        newProspectsCount={0}
        {...c}
      />,
    );
    expect(screen.getByText("Снять все")).toBeInTheDocument();
  });

  it("toggles an individual prospect", async () => {
    const c = cbs();
    render(
      <ProspectSelector
        prospects={[prospect({ id: "a" })]}
        selectedProspects={new Set()}
        selectedSeqId="seq1"
        launching={false}
        newProspectsCount={0}
        {...c}
      />,
    );
    await userEvent.click(screen.getByRole("checkbox"));
    expect(c.onToggle).toHaveBeenCalledWith("a");
  });

  it("hides the launch button when no sequence is selected", () => {
    render(
      <ProspectSelector
        prospects={[prospect({ id: "a" })]}
        selectedProspects={new Set(["a"])}
        selectedSeqId={null}
        launching={false}
        newProspectsCount={0}
        {...cbs()}
      />,
    );
    expect(screen.queryByText(/Запустить \(/)).not.toBeInTheDocument();
  });

  it("launches in two steps: first click reveals options, second fires onLaunch (send now by default)", async () => {
    const c = cbs();
    render(
      <ProspectSelector
        prospects={[prospect({ id: "a" }), prospect({ id: "b" })]}
        selectedProspects={new Set(["a", "b"])}
        selectedSeqId="seq1"
        launching={false}
        newProspectsCount={0}
        {...c}
      />,
    );
    const launch = screen.getByText(/Запустить \(2\)/);
    await userEvent.click(launch); // reveals options
    expect(screen.getByText("Режим запуска")).toBeInTheDocument();
    expect(c.onLaunch).not.toHaveBeenCalled();

    await userEvent.click(screen.getByText(/Запустить \(2\)/)); // fires
    expect(c.onLaunch).toHaveBeenCalledTimes(1);
    const [ids, sendNow] = c.onLaunch.mock.calls[0];
    expect(ids.sort()).toEqual(["a", "b"]);
    expect(sendNow).toBe(true);
  });

  it("respects the 'schedule' launch mode", async () => {
    const c = cbs();
    render(
      <ProspectSelector
        prospects={[prospect({ id: "a" })]}
        selectedProspects={new Set(["a"])}
        selectedSeqId="seq1"
        launching={false}
        newProspectsCount={0}
        {...c}
      />,
    );
    await userEvent.click(screen.getByText(/Запустить \(1\)/)); // reveal
    await userEvent.click(screen.getByText("Запланировать по расписанию"));
    await userEvent.click(screen.getByText(/Запустить \(1\)/)); // fire
    expect(c.onLaunch.mock.calls[0][1]).toBe(false);
  });

  it("shows a spinner and disables actions while launching", () => {
    render(
      <ProspectSelector
        prospects={[prospect({ id: "a" })]}
        selectedProspects={new Set(["a"])}
        selectedSeqId="seq1"
        launching
        newProspectsCount={0}
        {...cbs()}
      />,
    );
    expect(screen.getByText("Генерация...")).toBeInTheDocument();
  });

  it("launches for all new prospects", async () => {
    const c = cbs();
    render(
      <ProspectSelector
        prospects={[prospect({ id: "a" })]}
        selectedProspects={new Set()}
        selectedSeqId="seq1"
        launching={false}
        newProspectsCount={4}
        {...c}
      />,
    );
    await userEvent.click(screen.getByText(/Запустить для всех новых \(4\)/));
    expect(c.onLaunchAllNew).toHaveBeenCalledWith(true);
  });
});
