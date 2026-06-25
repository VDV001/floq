import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { Badge, badgeVariants } from "./badge";

describe("Badge", () => {
  it("renders children as a span by default", () => {
    render(<Badge>New</Badge>);
    const el = screen.getByText("New");
    expect(el.tagName).toBe("SPAN");
  });

  it("applies the default variant when none is passed", () => {
    render(<Badge>def</Badge>);
    expect(screen.getByText("def").className).toContain("bg-primary");
  });

  it.each([
    ["default", "bg-primary"],
    ["secondary", "bg-secondary"],
    ["destructive", "bg-destructive/10"],
    ["outline", "border-border"],
    ["ghost", "hover:bg-muted"],
    ["link", "underline-offset-4"],
  ] as const)("renders the %s variant with a distinctive token", (variant, token) => {
    render(<Badge variant={variant}>v</Badge>);
    expect(screen.getByText("v").className).toContain(token);
  });

  it("merges a custom className", () => {
    render(<Badge className="custom-y">y</Badge>);
    const el = screen.getByText("y");
    expect(el.className).toContain("custom-y");
    expect(el.className).toContain("bg-primary");
  });

  it("renders as a custom element via the render prop (asChild equivalent)", () => {
    render(<Badge render={<a href="/b" />}>linkbadge</Badge>);
    const link = screen.getByRole("link", { name: "linkbadge" });
    expect(link.tagName).toBe("A");
    expect(link).toHaveAttribute("href", "/b");
    expect(link.className).toContain("bg-primary");
  });

  it("badgeVariants helper produces variant tokens for direct use", () => {
    expect(badgeVariants({ variant: "outline" })).toContain("border-border");
  });
});
