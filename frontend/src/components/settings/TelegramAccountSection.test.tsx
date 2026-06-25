import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { TelegramAccountSection } from "./TelegramAccountSection";

type Props = React.ComponentProps<typeof TelegramAccountSection>;

function setup(over: Partial<Props> = {}) {
  const props: Props = {
    step: "idle",
    connectedPhone: "",
    phone: "",
    setPhone: vi.fn(),
    code: "",
    setCode: vi.fn(),
    loading: false,
    error: "",
    setError: vi.fn(),
    onSendCode: vi.fn(),
    onVerify: vi.fn(),
    onDisconnect: vi.fn(),
    onReset: vi.fn(),
    ...over,
  };
  render(<TelegramAccountSection {...props} />);
  return props;
}

describe("TelegramAccountSection — idle step", () => {
  it("prefixes a + when the typed number lacks one and strips invalid chars", () => {
    const props = setup({ phone: "" });
    fireEvent.change(screen.getByPlaceholderText("+7 999 123 4567"), { target: { value: "7abc999" } });
    expect(props.setPhone).toHaveBeenCalledWith("+7999");
    expect(props.setError).toHaveBeenCalledWith("");
  });

  it("keeps an existing leading + as-is", () => {
    const props = setup();
    fireEvent.change(screen.getByPlaceholderText("+7 999 123 4567"), { target: { value: "+7999" } });
    expect(props.setPhone).toHaveBeenCalledWith("+7999");
  });

  it("disables send when fewer than 10 digits", () => {
    setup({ phone: "+7999" });
    expect(screen.getByRole("button", { name: "Отправить код" })).toBeDisabled();
  });

  it("enables send for a full number and fires onSendCode", () => {
    const props = setup({ phone: "+79991234567" });
    const btn = screen.getByRole("button", { name: "Отправить код" });
    expect(btn).not.toBeDisabled();
    fireEvent.click(btn);
    expect(props.onSendCode).toHaveBeenCalledTimes(1);
  });

  it("shows the spinner label while loading", () => {
    setup({ phone: "+79991234567", loading: true });
    expect(screen.getByRole("button", { name: "..." })).toBeInTheDocument();
  });

  it("renders an inline error in the idle step", () => {
    setup({ error: "Неверный формат" });
    expect(screen.getByText("Неверный формат")).toBeInTheDocument();
  });
});

describe("TelegramAccountSection — code_sent step", () => {
  it("shows the phone the code was sent to and strips non-digits from the code", () => {
    const props = setup({ step: "code_sent", phone: "+79991234567" });
    expect(screen.getByText("+79991234567")).toBeInTheDocument();
    fireEvent.change(screen.getByPlaceholderText("Код из Telegram"), { target: { value: "12a3" } });
    expect(props.setCode).toHaveBeenCalledWith("123");
    expect(props.setError).toHaveBeenCalledWith("");
  });

  it("disables verify until the code is at least 4 chars", () => {
    setup({ step: "code_sent", code: "123" });
    expect(screen.getByRole("button", { name: "Подтвердить" })).toBeDisabled();
  });

  it("enables verify at 4 chars and fires onVerify", () => {
    const props = setup({ step: "code_sent", code: "1234" });
    fireEvent.click(screen.getByRole("button", { name: "Подтвердить" }));
    expect(props.onVerify).toHaveBeenCalledTimes(1);
  });

  it("offers a reset to enter another number", () => {
    const props = setup({ step: "code_sent" });
    fireEvent.click(screen.getByRole("button", { name: "Ввести другой номер" }));
    expect(props.onReset).toHaveBeenCalledTimes(1);
  });

  it("shows a verification error", () => {
    setup({ step: "code_sent", error: "Неверный код" });
    expect(screen.getByText("Неверный код")).toBeInTheDocument();
  });
});

describe("TelegramAccountSection — connected step", () => {
  it("shows the connected phone and a disconnect action", () => {
    const props = setup({ step: "connected", connectedPhone: "+79990001122" });
    expect(screen.getByText("+79990001122")).toBeInTheDocument();
    expect(screen.getByText("Подключен")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Отключить" }));
    expect(props.onDisconnect).toHaveBeenCalledTimes(1);
  });
});
