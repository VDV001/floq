import { test, expect } from "@playwright/test";
import { seedAuth, mockApi } from "./helpers";

// End-to-end check of the autopilot consolidation: the Outbound queue shows
// autopilot as a read-only status reflecting the auto_send setting, with the
// single control living on the Automations page.
test.describe("outbound autopilot status", () => {
  test("reads ВКЛ from settings and links to Automations to change it", async ({ page }) => {
    await seedAuth(page);
    await mockApi(page, { "/api/settings": { auto_send: true, auto_send_delay_min: 5 } });

    await page.goto("/outbound");

    await expect(page.getByRole("heading", { name: "Автопилот" })).toBeVisible();
    await expect(page.getByText("Вкл", { exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: /Настроить/ })).toHaveAttribute(
      "href",
      "/automations",
    );
  });

  test("shows ВЫКЛ when auto_send is off", async ({ page }) => {
    await seedAuth(page);
    await mockApi(page, { "/api/settings": { auto_send: false, auto_send_delay_min: 5 } });

    await page.goto("/outbound");

    await expect(page.getByText("Выкл", { exact: true })).toBeVisible();
  });
});
