import { test, expect } from "@playwright/test";
import { seedAuth, mockApi } from "./helpers";

// Mutation journey: approving a queued message removes it from the queue.
test.describe("outbound queue", () => {
  test("approving a message removes it from the queue", async ({ page }) => {
    const queued = {
      id: "msg-e2e-1",
      prospect_id: "abcdef123456",
      sequence_id: "seq-00000001",
      step_order: 1,
      channel: "email",
      body: "Тестовое письмо для проспекта",
      status: "draft",
      scheduled_at: "2026-06-25T10:00:00Z",
      sent_at: null,
      created_at: "2026-06-25T09:00:00Z",
    };

    await seedAuth(page);
    await mockApi(page, { "/api/outbound/queue": [queued] });

    await page.goto("/outbound");

    const card = page.getByText("Тестовое письмо для проспекта");
    await expect(card).toBeVisible();

    // exact — avoid colliding with the header's «Подтвердить все».
    await page.getByRole("button", { name: "Подтвердить", exact: true }).click();

    // handleApprove awaits the (mocked 200) approve, then drops the card from
    // the local queue — well before the 10s background poll could re-fetch.
    await expect(card).toHaveCount(0);
  });
});
