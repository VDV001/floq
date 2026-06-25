import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ScrollArea } from "./scroll-area";

describe("ScrollArea", () => {
  it("renders its children inside the viewport", () => {
    render(
      <ScrollArea>
        <p>scrollable body</p>
      </ScrollArea>
    );
    expect(screen.getByText("scrollable body")).toBeInTheDocument();
  });

  it("exposes the scroll-area data-slot on the root", () => {
    render(
      <ScrollArea data-testid="sa">
        <span>x</span>
      </ScrollArea>
    );
    expect(screen.getByTestId("sa")).toHaveAttribute("data-slot", "scroll-area");
  });

  it("merges a custom className with the relative base", () => {
    render(
      <ScrollArea className="h-40" data-testid="sa">
        <span>x</span>
      </ScrollArea>
    );
    const root = screen.getByTestId("sa");
    expect(root.className).toContain("h-40");
    expect(root.className).toContain("relative");
  });
});
