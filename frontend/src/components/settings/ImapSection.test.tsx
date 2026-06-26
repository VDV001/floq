import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ImapSection } from "./ImapSection";

type Props = React.ComponentProps<typeof ImapSection>;

function setup(over: Partial<Props> = {}) {
  const props: Props = {
    imapHost: "imap.gmail.com",
    setImapHost: vi.fn(),
    imapPort: "993",
    setImapPort: vi.fn(),
    imapUser: "user@example.com",
    setImapUser: vi.fn(),
    imapPassword: "",
    setImapPassword: vi.fn(),
    maskedPassword: "",
    active: false,
    testing: false,
    testResult: null,
    setTestResult: vi.fn(),
    onTest: vi.fn(),
    ...over,
  };
  render(<ImapSection {...props} />);
  return props;
}

describe("ImapSection", () => {
  it("propagates edits through every setter", () => {
    const props = setup();
    fireEvent.change(screen.getByDisplayValue("imap.gmail.com"), { target: { value: "imap.x.ru" } });
    expect(props.setImapHost).toHaveBeenCalledWith("imap.x.ru");
    fireEvent.change(screen.getByDisplayValue("993"), { target: { value: "143" } });
    expect(props.setImapPort).toHaveBeenCalledWith("143");
    fireEvent.change(screen.getByDisplayValue("user@example.com"), { target: { value: "new@x.ru" } });
    expect(props.setImapUser).toHaveBeenCalledWith("new@x.ru");
  });

  it("shows the masked password as placeholder when set", () => {
    setup({ maskedPassword: "ab••cd" });
    expect(screen.getByPlaceholderText("ab••cd")).toBeInTheDocument();
  });

  it("falls back to a dotted placeholder when no masked password", () => {
    setup({ maskedPassword: "" });
    expect(screen.getByPlaceholderText("••••••••••••")).toBeInTheDocument();
  });

  it("shows the connected badge when active", () => {
    setup({ active: true });
    expect(screen.getByText("Подключен")).toBeInTheDocument();
  });

  it("shows the disconnected badge when inactive", () => {
    setup({ active: false });
    expect(screen.getByText("Не подключен")).toBeInTheDocument();
  });

  it("disables the test button and shows progress text while testing", () => {
    setup({ testing: true });
    const btn = screen.getByRole("button", { name: /Проверяем/ });
    expect(btn).toBeDisabled();
  });

  it("fires onTest when the test button is clicked", () => {
    const props = setup();
    fireEvent.click(screen.getByRole("button", { name: /Тест соединения/ }));
    expect(props.onTest).toHaveBeenCalledTimes(1);
  });

  it("renders a success banner and dismisses it", () => {
    const props = setup({ testResult: { success: true, message: "OK!" } });
    expect(screen.getByText("OK!")).toBeInTheDocument();
    const dismiss = screen.getAllByRole("button").find((b) => b.textContent === "")!;
    fireEvent.click(dismiss);
    expect(props.setTestResult).toHaveBeenCalledWith(null);
  });

  it("renders an error banner with the error text", () => {
    setup({ testResult: { success: false, error: "Ошибка авторизации" } });
    expect(screen.getByText("Ошибка авторизации")).toBeInTheDocument();
  });
});
