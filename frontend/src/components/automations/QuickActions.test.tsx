import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { QuickActions } from "./QuickActions";

describe("QuickActions", () => {
  it("shows the all-on state when every toggle is enabled", () => {
    render(
      <QuickActions
        toggles={{ a: true, b: true }}
        onToggleAll={vi.fn()}
      />
    );
    expect(
      screen.getByText(/Все автоматизации включены/)
    ).toBeInTheDocument();
    expect(screen.getByText("Выключить все")).toBeInTheDocument();
  });

  it("shows the partial state with the enabled count when not all on", () => {
    render(
      <QuickActions
        toggles={{ a: true, b: false, c: false }}
        onToggleAll={vi.fn()}
      />
    );
    // 1 enabled of 6 total automations.
    expect(screen.getByText(/Включено 1 из 6 автоматизаций/)).toBeInTheDocument();
    expect(screen.getByText("Включить все")).toBeInTheDocument();
  });

  it("fires onToggleAll when the button is clicked", () => {
    const onToggleAll = vi.fn();
    render(
      <QuickActions
        toggles={{ a: false }}
        onToggleAll={onToggleAll}
      />
    );
    fireEvent.click(screen.getByText("Включить все"));
    expect(onToggleAll).toHaveBeenCalledTimes(1);
  });
});
