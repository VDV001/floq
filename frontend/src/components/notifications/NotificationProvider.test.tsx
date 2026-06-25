import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { NotificationProvider, useNotify } from "./NotificationProvider";
import { ApiError } from "@/lib/api";

function Trigger({ onReady }: { onReady?: (n: ReturnType<typeof useNotify>) => void }) {
  const notify = useNotify();
  onReady?.(notify);
  return (
    <div>
      <button onClick={() => notify.notify({ type: "success", title: "Готово", message: "Сохранено" })}>
        ok
      </button>
      <button
        onClick={() =>
          notify.notify({
            type: "error",
            title: "Ошибка",
            message: "Что-то сломалось",
            remedy: "Попробуйте ещё раз",
          })
        }
      >
        err
      </button>
      <button
        onClick={() =>
          notify.notifyError(
            new ApiError("ИИ не подключён", 503, "ai_not_configured", "Откройте Настройки → ИИ"),
          )
        }
      >
        apierr
      </button>
      <button
        onClick={() =>
          notify.notifyError(
            new ApiError("Почта не подключена", 400, "email_not_configured", "Откройте Настройки → Почта"),
          )
        }
      >
        emailerr
      </button>
    </div>
  );
}

function renderWithProvider() {
  return render(
    <NotificationProvider>
      <Trigger />
    </NotificationProvider>,
  );
}

describe("NotificationProvider", () => {
  it("renders children", () => {
    renderWithProvider();
    expect(screen.getByRole("button", { name: "ok" })).toBeInTheDocument();
  });

  it("shows a notification with its message when notify() is called", async () => {
    const user = userEvent.setup();
    renderWithProvider();

    await user.click(screen.getByRole("button", { name: "ok" }));

    expect(screen.getByText("Сохранено")).toBeInTheDocument();
    expect(screen.getByText("Готово")).toBeInTheDocument();
  });

  it("shows the remedy text ('what to do') for an error notification", async () => {
    const user = userEvent.setup();
    renderWithProvider();

    await user.click(screen.getByRole("button", { name: "err" }));

    expect(screen.getByText("Что-то сломалось")).toBeInTheDocument();
    expect(screen.getByText("Попробуйте ещё раз")).toBeInTheDocument();
  });

  it("maps an ApiError to a notification carrying its message and remedy", async () => {
    const user = userEvent.setup();
    renderWithProvider();

    await user.click(screen.getByRole("button", { name: "apierr" }));

    expect(screen.getByText("ИИ не подключён")).toBeInTheDocument();
    expect(screen.getByText("Откройте Настройки → ИИ")).toBeInTheDocument();
  });

  it("offers a Settings link for the email_not_configured code", async () => {
    const user = userEvent.setup();
    renderWithProvider();

    await user.click(screen.getByRole("button", { name: "emailerr" }));

    expect(screen.getByText("Почта не подключена")).toBeInTheDocument();
    const link = screen.getByRole("link", { name: /Открыть настройки почты/ });
    expect(link).toHaveAttribute("href", "/settings");
  });

  it("dismisses a notification when its close button is clicked", async () => {
    const user = userEvent.setup();
    renderWithProvider();

    await user.click(screen.getByRole("button", { name: "err" }));
    expect(screen.getByText("Что-то сломалось")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /закрыть уведомление/i }));

    expect(screen.queryByText("Что-то сломалось")).not.toBeInTheDocument();
  });

  it("throws if useNotify is used outside the provider", () => {
    function Orphan() {
      useNotify();
      return null;
    }
    expect(() => render(<Orphan />)).toThrow(/NotificationProvider/);
  });
});
