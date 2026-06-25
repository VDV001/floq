import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { PeriodSelector } from "./PeriodSelector";

describe("PeriodSelector", () => {
  it("renders one radio per option and marks the selected one as checked", () => {
    render(<PeriodSelector value="month" onChange={vi.fn()} />);
    expect(screen.getByRole("radio", { name: "Неделя" })).toHaveAttribute("aria-checked", "false");
    expect(screen.getByRole("radio", { name: "Месяц" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("radio", { name: "Всё время" })).toHaveAttribute("aria-checked", "false");
  });

  it("calls onChange with the option value when clicked", () => {
    const onChange = vi.fn();
    render(<PeriodSelector value="month" onChange={onChange} />);
    fireEvent.click(screen.getByRole("radio", { name: "Неделя" }));
    expect(onChange).toHaveBeenCalledWith("week");
    fireEvent.click(screen.getByRole("radio", { name: "Всё время" }));
    expect(onChange).toHaveBeenCalledWith("all");
  });
});
