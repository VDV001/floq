import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import LoginPage from "./page";

const pushMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock, back: vi.fn() }),
  usePathname: () => "/login",
}));

vi.mock("@/lib/api", () => ({
  api: {
    login: vi.fn(),
    register: vi.fn(),
  },
}));

import { api } from "@/lib/api";

describe("LoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  it("renders login form with email, password and submit", () => {
    render(<LoginPage />);
    expect(screen.getByText("Вход в Floq")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("name@company.com")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("••••••••")).toBeInTheDocument();
    expect(screen.getByText("Войти")).toBeInTheDocument();
  });

  it("allows typing email and password", async () => {
    const user = userEvent.setup();
    render(<LoginPage />);

    const emailInput = screen.getByPlaceholderText("name@company.com");
    const passwordInput = screen.getByPlaceholderText("••••••••");

    await user.type(emailInput, "test@example.com");
    await user.type(passwordInput, "password123");

    expect(emailInput).toHaveValue("test@example.com");
    expect(passwordInput).toHaveValue("password123");
  });

  it("submits login and redirects on success", async () => {
    const user = userEvent.setup();
    vi.mocked(api.login).mockResolvedValue({
      token: "tok123",
      refresh_token: "ref123",
    });

    render(<LoginPage />);

    await user.type(screen.getByPlaceholderText("name@company.com"), "test@example.com");
    await user.type(screen.getByPlaceholderText("••••••••"), "password123");
    await user.click(screen.getByText("Войти"));

    await waitFor(() => {
      expect(api.login).toHaveBeenCalledWith("test@example.com", "password123");
      expect(localStorage.getItem("token")).toBe("tok123");
      expect(pushMock).toHaveBeenCalledWith("/inbox");
    });
  });

  it("shows error on login failure", async () => {
    const user = userEvent.setup();
    vi.mocked(api.login).mockRejectedValue(new Error("invalid"));

    render(<LoginPage />);

    await user.type(screen.getByPlaceholderText("name@company.com"), "bad@example.com");
    await user.type(screen.getByPlaceholderText("••••••••"), "wrong123");
    await user.click(screen.getByText("Войти"));

    await waitFor(() => {
      expect(screen.getByText("Неверный email или пароль")).toBeInTheDocument();
    });
  });

  it("can switch to register mode", async () => {
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByText("Зарегистрироваться"));

    expect(screen.getByRole("heading", { name: "Создать аккаунт" })).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Иван Иванов")).toBeInTheDocument();
  });
});
