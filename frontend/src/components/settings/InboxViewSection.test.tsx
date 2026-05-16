import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { InboxViewSection } from "./InboxViewSection";

describe("InboxViewSection", () => {
  it("renders the checkbox in the supplied state", () => {
    render(<InboxViewSection aggregated={true} saving={false} onToggle={() => {}} />);
    const checkbox = screen.getByRole("checkbox") as HTMLInputElement;
    expect(checkbox.checked).toBe(true);
  });

  it("invokes onToggle with the new value when clicked", () => {
    const onToggle = vi.fn();
    render(<InboxViewSection aggregated={true} saving={false} onToggle={onToggle} />);
    fireEvent.click(screen.getByRole("checkbox"));
    expect(onToggle).toHaveBeenCalledWith(false);
  });

  it("disables the checkbox while a save is in flight", () => {
    render(<InboxViewSection aggregated={true} saving={true} onToggle={() => {}} />);
    const checkbox = screen.getByRole("checkbox") as HTMLInputElement;
    expect(checkbox.disabled).toBe(true);
  });

  it("exposes the help text via aria-describedby", () => {
    render(<InboxViewSection aggregated={false} saving={false} onToggle={() => {}} />);
    const checkbox = screen.getByRole("checkbox");
    const describedBy = checkbox.getAttribute("aria-describedby");
    expect(describedBy).toBeTruthy();
    expect(document.getElementById(describedBy!)).toBeTruthy();
  });
});
