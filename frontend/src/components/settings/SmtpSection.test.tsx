import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { SmtpSection } from "./SmtpSection";

type Props = React.ComponentProps<typeof SmtpSection>;

function setup(over: Partial<Props> = {}) {
  const props: Props = {
    smtpHost: "smtp.mail.ru",
    setSmtpHost: vi.fn(),
    smtpPort: "465",
    setSmtpPort: vi.fn(),
    smtpUser: "hello@x.ru",
    setSmtpUser: vi.fn(),
    smtpPassword: "",
    setSmtpPassword: vi.fn(),
    maskedPassword: "",
    active: false,
    testing: false,
    testResult: null,
    setTestResult: vi.fn(),
    onTest: vi.fn(),
    ...over,
  };
  render(<SmtpSection {...props} />);
  return props;
}

describe("SmtpSection", () => {
  it("propagates edits through every setter", () => {
    const props = setup();
    fireEvent.change(screen.getByDisplayValue("smtp.mail.ru"), { target: { value: "smtp.yandex.ru" } });
    expect(props.setSmtpHost).toHaveBeenCalledWith("smtp.yandex.ru");
    fireEvent.change(screen.getByDisplayValue("465"), { target: { value: "587" } });
    expect(props.setSmtpPort).toHaveBeenCalledWith("587");
    fireEvent.change(screen.getByDisplayValue("hello@x.ru"), { target: { value: "new@x.ru" } });
    expect(props.setSmtpUser).toHaveBeenCalledWith("new@x.ru");
  });

  it("uses the masked password as placeholder, falling back to dots", () => {
    setup({ maskedPassword: "se••et" });
    expect(screen.getByPlaceholderText("se••et")).toBeInTheDocument();
  });

  it("shows connected / disconnected badges by active flag", () => {
    const { rerender } = render(
      <SmtpSection
        smtpHost="" setSmtpHost={vi.fn()} smtpPort="" setSmtpPort={vi.fn()}
        smtpUser="" setSmtpUser={vi.fn()} smtpPassword="" setSmtpPassword={vi.fn()}
        maskedPassword="" active={true} testing={false} testResult={null}
        setTestResult={vi.fn()} onTest={vi.fn()}
      />,
    );
    expect(screen.getByText("Подключен")).toBeInTheDocument();
    rerender(
      <SmtpSection
        smtpHost="" setSmtpHost={vi.fn()} smtpPort="" setSmtpPort={vi.fn()}
        smtpUser="" setSmtpUser={vi.fn()} smtpPassword="" setSmtpPassword={vi.fn()}
        maskedPassword="" active={false} testing={false} testResult={null}
        setTestResult={vi.fn()} onTest={vi.fn()}
      />,
    );
    expect(screen.getByText("Не подключен")).toBeInTheDocument();
  });

  it("disables the button while testing", () => {
    setup({ testing: true });
    expect(screen.getByRole("button", { name: /Проверяем/ })).toBeDisabled();
  });

  it("fires onTest on click", () => {
    const props = setup();
    fireEvent.click(screen.getByRole("button", { name: /Тест соединения/ }));
    expect(props.onTest).toHaveBeenCalledTimes(1);
  });

  it("renders error banner text", () => {
    setup({ testResult: { success: false, error: "Connection refused" } });
    expect(screen.getByText("Connection refused")).toBeInTheDocument();
  });
});
