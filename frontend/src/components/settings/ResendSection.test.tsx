import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ResendSection } from "./ResendSection";

type Props = React.ComponentProps<typeof ResendSection>;

function setup(over: Partial<Props> = {}) {
  const props: Props = {
    maskedKey: "",
    resendKey: "",
    setResendKey: vi.fn(),
    active: false,
    testing: false,
    testResult: null,
    setTestResult: vi.fn(),
    hasKey: false,
    onTest: vi.fn(),
    ...over,
  };
  render(<ResendSection {...props} />);
  return props;
}

describe("ResendSection", () => {
  it("propagates key edits", () => {
    const props = setup();
    fireEvent.change(screen.getByPlaceholderText("re_123456789..."), { target: { value: "re_abc" } });
    expect(props.setResendKey).toHaveBeenCalledWith("re_abc");
  });

  it("uses the masked key as placeholder when present", () => {
    setup({ maskedKey: "re_••cd" });
    expect(screen.getByPlaceholderText("re_••cd")).toBeInTheDocument();
  });

  it("disables the test button when there is neither a typed key nor a stored key", () => {
    setup({ resendKey: "", hasKey: false });
    expect(screen.getByRole("button", { name: "Проверить" })).toBeDisabled();
  });

  it("enables the test button when a key is typed", () => {
    setup({ resendKey: "re_typed", hasKey: false });
    expect(screen.getByRole("button", { name: "Проверить" })).not.toBeDisabled();
  });

  it("enables the test button when a stored key exists", () => {
    setup({ resendKey: "", hasKey: true });
    expect(screen.getByRole("button", { name: "Проверить" })).not.toBeDisabled();
  });

  it("disables the button while testing even with a key", () => {
    setup({ resendKey: "re_typed", testing: true });
    expect(screen.getByRole("button")).toBeDisabled();
  });

  it("fires onTest on click", () => {
    const props = setup({ hasKey: true });
    fireEvent.click(screen.getByRole("button", { name: "Проверить" }));
    expect(props.onTest).toHaveBeenCalledTimes(1);
  });

  it("shows the connected badge and a success banner", () => {
    setup({ active: true, testResult: { success: true, message: "Ключ валиден" } });
    expect(screen.getByText("Подключен")).toBeInTheDocument();
    expect(screen.getByText("Ключ валиден")).toBeInTheDocument();
  });
});
