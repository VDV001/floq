import { test, expect } from "@playwright/test";
import { seedAuth, mockApi } from "./helpers";

test.describe("dashboard navigation", () => {
  test.beforeEach(async ({ page }) => {
    await seedAuth(page);
    await mockApi(page);
  });

  test("sidebar shows the disambiguated labels (Напоминания, not Лиды)", async ({ page }) => {
    await page.goto("/inbox");
    const nav = page.getByRole("navigation");
    await expect(nav.getByRole("link", { name: "Напоминания" })).toBeVisible();
    await expect(nav.getByRole("link", { name: "Воронка" })).toBeVisible();
    // The old misleading «Лиды» entry is gone.
    await expect(nav.getByRole("link", { name: "Лиды", exact: true })).toHaveCount(0);
  });

  test("analytics funnel is titled «Конверсия», not «Воронка»", async ({ page }) => {
    await page.goto("/analytics/funnel");
    await expect(page.getByRole("heading", { name: "Конверсия", exact: true })).toBeVisible();
    // Sub-nav tab renamed too.
    await expect(page.getByRole("link", { name: "Конверсия", exact: true })).toBeVisible();
  });
});
