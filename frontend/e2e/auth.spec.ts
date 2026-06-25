import { test, expect } from "@playwright/test";
import { mockApi } from "./helpers";

test.describe("authentication", () => {
  test("redirects to /login when there is no token", async ({ page }) => {
    await mockApi(page);
    await page.goto("/inbox");
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByRole("heading", { name: "Вход в Floq" })).toBeVisible();
  });

  test("logs in and lands on the inbox", async ({ page }) => {
    await mockApi(page, {
      "/api/auth/login": { token: "e2e-token", refresh_token: "e2e-refresh" },
    });
    await page.goto("/login");

    await page.getByPlaceholder("name@company.com").fill("user@floq.dev");
    await page.getByPlaceholder("••••••••").fill("secret123");
    await page.getByRole("button", { name: "Войти" }).click();

    await expect(page).toHaveURL(/\/inbox$/);
    // The main sidebar nav confirms we're inside the authenticated shell
    // (the «Floq» logo also appears on /login, so it's not a reliable marker).
    await expect(page.getByRole("navigation", { name: "Основная навигация" })).toBeVisible();
  });

  test("shows an error and stays on /login when credentials are rejected", async ({ page }) => {
    await mockApi(page);
    // Registered after mockApi → takes precedence for this path (LIFO).
    await page.route("**/api/auth/login", (route) =>
      route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ error: "invalid" }) }),
    );
    await page.goto("/login");

    await page.getByPlaceholder("name@company.com").fill("user@floq.dev");
    await page.getByPlaceholder("••••••••").fill("wrongpass");
    await page.getByRole("button", { name: "Войти" }).click();

    await expect(page.getByText("Неверный email или пароль")).toBeVisible();
    await expect(page).toHaveURL(/\/login$/);
  });
});
