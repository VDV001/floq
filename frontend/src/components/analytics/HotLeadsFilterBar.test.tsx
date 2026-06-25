import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { HotLeadsFilterBar } from "./HotLeadsFilterBar";

describe("HotLeadsFilterBar", () => {
  it("reflects the selected status and channel values", () => {
    render(
      <HotLeadsFilterBar
        status="qualified"
        channel="telegram"
        onStatusChange={vi.fn()}
        onChannelChange={vi.fn()}
      />,
    );
    expect((screen.getByLabelText("Статус") as HTMLSelectElement).value).toBe("qualified");
    expect((screen.getByLabelText("Канал") as HTMLSelectElement).value).toBe("telegram");
  });

  it("calls onStatusChange with the chosen status value", () => {
    const onStatusChange = vi.fn();
    render(
      <HotLeadsFilterBar
        status="any"
        channel="any"
        onStatusChange={onStatusChange}
        onChannelChange={vi.fn()}
      />,
    );
    fireEvent.change(screen.getByLabelText("Статус"), { target: { value: "closed" } });
    expect(onStatusChange).toHaveBeenCalledWith("closed");
  });

  it("calls onChannelChange with the chosen channel value", () => {
    const onChannelChange = vi.fn();
    render(
      <HotLeadsFilterBar
        status="any"
        channel="any"
        onStatusChange={vi.fn()}
        onChannelChange={onChannelChange}
      />,
    );
    fireEvent.change(screen.getByLabelText("Канал"), { target: { value: "email" } });
    expect(onChannelChange).toHaveBeenCalledWith("email");
  });
});
