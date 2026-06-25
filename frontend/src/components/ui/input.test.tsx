import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { Input } from "./input";

describe("Input", () => {
  it("renders with the input data-slot", () => {
    render(<Input placeholder="email" />);
    const el = screen.getByPlaceholderText("email");
    expect(el).toHaveAttribute("data-slot", "input");
  });

  it("forwards the type attribute", () => {
    render(<Input type="password" placeholder="pw" />);
    expect(screen.getByPlaceholderText("pw")).toHaveAttribute("type", "password");
  });

  it("propagates onChange with the typed value", () => {
    const onChange = vi.fn();
    render(<Input placeholder="name" onChange={onChange} />);
    const el = screen.getByPlaceholderText("name") as HTMLInputElement;
    fireEvent.change(el, { target: { value: "hello" } });
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(el.value).toBe("hello");
  });

  it("merges a custom className with the base classes", () => {
    render(<Input className="my-input" placeholder="x" />);
    const el = screen.getByPlaceholderText("x");
    expect(el.className).toContain("my-input");
    expect(el.className).toContain("w-full");
  });

  it("honors the disabled attribute", () => {
    render(<Input placeholder="d" disabled />);
    expect(screen.getByPlaceholderText("d")).toBeDisabled();
  });
});
