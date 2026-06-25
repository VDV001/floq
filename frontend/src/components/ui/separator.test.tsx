import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { Separator } from "./separator";

describe("Separator", () => {
  it("renders with the separator data-slot and defaults to horizontal", () => {
    render(<Separator data-testid="sep" />);
    const sep = screen.getByTestId("sep");
    expect(sep).toHaveAttribute("data-slot", "separator");
    expect(sep).toHaveAttribute("data-orientation", "horizontal");
  });

  it("renders a vertical orientation when requested", () => {
    render(<Separator orientation="vertical" data-testid="sep" />);
    expect(screen.getByTestId("sep")).toHaveAttribute(
      "data-orientation",
      "vertical"
    );
  });

  it("merges a custom className with the base classes", () => {
    render(<Separator className="my-sep" data-testid="sep" />);
    const sep = screen.getByTestId("sep");
    expect(sep.className).toContain("my-sep");
    expect(sep.className).toContain("bg-border");
  });
});
