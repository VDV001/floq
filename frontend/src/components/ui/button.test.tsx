import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { Button, buttonVariants } from "./button";

describe("Button", () => {
  it("renders children with the button data-slot by default", () => {
    render(<Button>Click me</Button>);
    const btn = screen.getByRole("button", { name: "Click me" });
    expect(btn).toBeInTheDocument();
    expect(btn).toHaveAttribute("data-slot", "button");
  });

  it("applies the default variant + size classes when none are passed", () => {
    render(<Button>Default</Button>);
    const btn = screen.getByRole("button");
    expect(btn.className).toContain("bg-primary");
    expect(btn.className).toContain("h-8");
  });

  it.each([
    ["default", "bg-primary"],
    ["outline", "border-border"],
    ["secondary", "bg-secondary"],
    ["ghost", "hover:bg-muted"],
    ["destructive", "bg-destructive/10"],
    ["link", "underline-offset-4"],
  ] as const)("renders the %s variant with a distinctive token", (variant, token) => {
    render(<Button variant={variant}>v</Button>);
    expect(screen.getByRole("button").className).toContain(token);
  });

  it.each([
    ["default", "h-8"],
    ["xs", "h-6"],
    ["sm", "h-7"],
    ["lg", "h-9"],
    ["icon", "size-8"],
    ["icon-xs", "size-6"],
    ["icon-sm", "size-7"],
    ["icon-lg", "size-9"],
  ] as const)("renders the %s size with a distinctive token", (size, token) => {
    render(<Button size={size}>s</Button>);
    expect(screen.getByRole("button").className).toContain(token);
  });

  it("merges a custom className alongside the variant classes", () => {
    render(<Button className="custom-x">x</Button>);
    const btn = screen.getByRole("button");
    expect(btn.className).toContain("custom-x");
    expect(btn.className).toContain("bg-primary");
  });

  it("fires onClick when clicked", () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>go</Button>);
    fireEvent.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("does not fire onClick when disabled", () => {
    const onClick = vi.fn();
    render(
      <Button onClick={onClick} disabled>
        nope
      </Button>
    );
    fireEvent.click(screen.getByRole("button"));
    expect(onClick).not.toHaveBeenCalled();
  });

  it("renders as a custom element via the render prop (asChild equivalent)", () => {
    render(<Button render={<a href="/go" />}>link</Button>);
    const link = screen.getByRole("link", { name: "link" });
    expect(link.tagName).toBe("A");
    expect(link).toHaveAttribute("href", "/go");
    expect(link).toHaveAttribute("data-slot", "button");
  });

  it("buttonVariants helper produces variant + size tokens for direct use", () => {
    const cls = buttonVariants({ variant: "outline", size: "lg" });
    expect(cls).toContain("border-border");
    expect(cls).toContain("h-9");
  });
});
