import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock, back: vi.fn() }),
  usePathname: () => "/login",
}));

import LoginPage from "./page";

// Integration: real LoginPage -> real lib/api.ts -> network (MSW). Nothing
// from @/lib/api is stubbed, so this exercises apiFetch, token persistence,
// and the page's submit/redirect wiring together.
describe("login flow (integration)", () => {
  beforeEach(() => {
    pushMock.mockReset();
  });

  it("logs in, persists tokens via the real api client, and redirects", async () => {
    const user = userEvent.setup({ delay: null });
    let receivedBody: { email?: string; password?: string } = {};
    server.use(
      http.post(url("/api/auth/login"), async ({ request }) => {
        receivedBody = (await request.json()) as typeof receivedBody;
        return HttpResponse.json({ token: "tok-123", refresh_token: "ref-456" });
      }),
    );

    render(<LoginPage />);
    await user.type(screen.getByPlaceholderText("name@company.com"), "user@floq.io");
    await user.type(screen.getByPlaceholderText("••••••••"), "secret123");
    await user.click(screen.getByRole("button", { name: "Войти" }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/inbox"));
    expect(receivedBody).toEqual({ email: "user@floq.io", password: "secret123" });
    expect(localStorage.getItem("token")).toBe("tok-123");
    expect(localStorage.getItem("refresh_token")).toBe("ref-456");
  });

  it("shows an error and does not redirect when the backend rejects credentials", async () => {
    const user = userEvent.setup({ delay: null });
    server.use(
      http.post(url("/api/auth/login"), () =>
        HttpResponse.json({ error: "invalid" }, { status: 401 }),
      ),
    );

    render(<LoginPage />);
    await user.type(screen.getByPlaceholderText("name@company.com"), "user@floq.io");
    await user.type(screen.getByPlaceholderText("••••••••"), "wrongpass");
    await user.click(screen.getByRole("button", { name: "Войти" }));

    expect(await screen.findByText("Неверный email или пароль")).toBeInTheDocument();
    expect(pushMock).not.toHaveBeenCalled();
    expect(localStorage.getItem("token")).toBeNull();
  });

  it("registers a new account through the register endpoint", async () => {
    const user = userEvent.setup({ delay: null });
    server.use(
      http.post(url("/api/auth/register"), () =>
        HttpResponse.json({ token: "rt", refresh_token: "rr" }),
      ),
    );

    render(<LoginPage />);
    // Switch to register mode via the toggle link.
    await user.click(screen.getByText("Зарегистрироваться"));
    await user.type(screen.getByPlaceholderText("Иван Иванов"), "Новый Пользователь");
    await user.type(screen.getByPlaceholderText("name@company.com"), "new@floq.io");
    await user.type(screen.getByPlaceholderText("••••••••"), "Strong1!");
    await user.click(screen.getByRole("button", { name: "Создать аккаунт" }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/inbox"));
    expect(localStorage.getItem("token")).toBe("rt");
  });
});
