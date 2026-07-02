import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ModelCombobox } from "./model-combobox";
import type { AIModelOption } from "@/lib/api";

const opts: AIModelOption[] = [
  { id: "gpt-4o" },
  { id: "gpt-4o-mini" },
  { id: "gemma3:4b", meta: "4B" },
];

function setup(over: Partial<Parameters<typeof ModelCombobox>[0]> = {}) {
  const onChange = vi.fn();
  render(<ModelCombobox value="" onChange={onChange} options={opts} {...over} />);
  return { onChange };
}

describe("ModelCombobox (#229)", () => {
  it("shows the current value on the trigger", () => {
    setup({ value: "gpt-4o" });
    expect(screen.getByRole("button")).toHaveTextContent("gpt-4o");
  });

  it("opens and lists options", () => {
    setup();
    fireEvent.click(screen.getByRole("button"));
    expect(screen.getByText("gpt-4o")).toBeInTheDocument();
    expect(screen.getByText("gpt-4o-mini")).toBeInTheDocument();
  });

  it("filters options as you type (typeahead)", () => {
    setup();
    fireEvent.click(screen.getByRole("button"));
    fireEvent.change(screen.getByPlaceholderText(/Поиск/), { target: { value: "mini" } });
    expect(screen.queryByText("gpt-4o-mini")).toBeInTheDocument();
    // "gpt-4o" (exact, no "mini") should be filtered out of the option rows
    expect(screen.queryByRole("option", { name: /^gpt-4o$/ })).not.toBeInTheDocument();
  });

  it("selects an option on click", () => {
    const { onChange } = setup();
    fireEvent.click(screen.getByRole("button"));
    fireEvent.click(screen.getByText("gpt-4o-mini"));
    expect(onChange).toHaveBeenCalledWith("gpt-4o-mini");
  });

  it("navigates with arrows and selects with Enter", () => {
    const { onChange } = setup();
    fireEvent.click(screen.getByRole("button"));
    const search = screen.getByPlaceholderText(/Поиск/);
    fireEvent.keyDown(search, { key: "ArrowDown" }); // highlight first
    fireEvent.keyDown(search, { key: "ArrowDown" }); // highlight second
    fireEvent.keyDown(search, { key: "Enter" });
    expect(onChange).toHaveBeenCalledWith("gpt-4o-mini");
  });

  it("accepts free-text custom model as fallback", () => {
    const { onChange } = setup();
    fireEvent.click(screen.getByRole("button"));
    const search = screen.getByPlaceholderText(/Поиск/);
    fireEvent.change(search, { target: { value: "my-custom-model" } });
    fireEvent.keyDown(search, { key: "Enter" });
    expect(onChange).toHaveBeenCalledWith("my-custom-model");
  });

  it("shows meta next to a model when present", () => {
    setup();
    fireEvent.click(screen.getByRole("button"));
    expect(screen.getByText("4B")).toBeInTheDocument();
  });
});
