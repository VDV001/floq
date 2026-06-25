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
    // Sidebar logo confirms we're inside the authenticated dashboard shell.
    await expect(page.getByRole("heading", { name: "Floq" })).toBeVisible();
  });
});
