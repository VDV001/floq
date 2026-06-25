import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, beforeEach, vi } from "vitest";
import { Tabs, TabsList, TabsTrigger, TabsContent, tabsListVariants } from "./tabs";

beforeEach(() => {
  // Radix/base-ui tab activation relies on pointer capture + scrolling the
  // active tab into view, neither of which jsdom implements.
  Element.prototype.scrollIntoView = vi.fn();
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = vi.fn(() => false);
  }
});

function Fixture({ defaultValue = "a" }: { defaultValue?: string }) {
  return (
    <Tabs defaultValue={defaultValue}>
      <TabsList>
        <TabsTrigger value="a">Tab A</TabsTrigger>
        <TabsTrigger value="b">Tab B</TabsTrigger>
      </TabsList>
      <TabsContent value="a">Panel A</TabsContent>
      <TabsContent value="b">Panel B</TabsContent>
    </Tabs>
  );
}

describe("Tabs", () => {
  it("shows the default tab's panel content", () => {
    render(<Fixture />);
    expect(screen.getByText("Panel A")).toBeInTheDocument();
  });

  it("switches the active panel when a different tab is clicked", () => {
    render(<Fixture />);
    fireEvent.click(screen.getByRole("tab", { name: "Tab B" }));
    expect(screen.getByText("Panel B")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Tab B" })).toHaveAttribute(
      "data-active",
      ""
    );
  });

  it("marks the active trigger with data-active", () => {
    render(<Fixture />);
    expect(screen.getByRole("tab", { name: "Tab A" })).toHaveAttribute(
      "data-active",
      ""
    );
  });

  it("sets data-slot attributes on the parts", () => {
    render(<Fixture />);
    expect(screen.getByRole("tablist")).toHaveAttribute(
      "data-slot",
      "tabs-list"
    );
    expect(screen.getByRole("tab", { name: "Tab A" })).toHaveAttribute(
      "data-slot",
      "tabs-trigger"
    );
  });

  it.each([
    ["default", "bg-muted"],
    ["line", "bg-transparent"],
  ] as const)("TabsList renders the %s variant", (variant, token) => {
    render(
      <Tabs defaultValue="a">
        <TabsList variant={variant}>
          <TabsTrigger value="a">A</TabsTrigger>
        </TabsList>
        <TabsContent value="a">P</TabsContent>
      </Tabs>
    );
    const list = screen.getByRole("tablist");
    expect(list).toHaveAttribute("data-variant", variant);
    expect(list.className).toContain(token);
  });

  it("tabsListVariants helper exposes variant tokens", () => {
    expect(tabsListVariants({ variant: "default" })).toContain("bg-muted");
    expect(tabsListVariants({ variant: "line" })).toContain("bg-transparent");
  });
});
