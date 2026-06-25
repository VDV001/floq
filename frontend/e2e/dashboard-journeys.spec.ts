import { test, expect } from "@playwright/test";
import { seedAuth, mockApi } from "./helpers";

// Critical-journey coverage for the main dashboard surfaces. Each journey
// seeds auth + mocks exactly the endpoints its page hits on mount (the
// mockApi helper supplies sane defaults for the shared sidebar fetches), then
// asserts the page reached its data-rendered state. No real backend.

test.describe("prospects journey", () => {
  test("loads the outbound base and renders contacts with the count", async ({ page }) => {
    await seedAuth(page);
    await mockApi(page, {
      "/api/sources": [],
      "/api/prospects": [
        {
          id: "p-1", user_id: "u-1", name: "Иван Петров", company: "Acme", title: "CEO",
          email: "ivan@acme.io", phone: "", whatsapp: "", telegram_username: "",
          industry: "", company_size: "", context: "", source: "csv", source_name: "Источник A",
          status: "new", consent_status: "none", verify_status: "not_checked", verify_score: 0,
          verify_details: {}, verified_at: null, converted_lead_id: null,
          created_at: "2026-06-01T00:00:00Z", updated_at: "2026-06-01T00:00:00Z",
        },
      ],
    });

    await page.goto("/prospects");

    await expect(page.getByRole("heading", { name: "Проспекты" })).toBeVisible();
    await expect(page.getByText("Иван Петров")).toBeVisible();
    await expect(page.getByText("1 контактов")).toBeVisible();
  });
});

test.describe("sequences journey", () => {
  test("loads a sequence and shows its steps", async ({ page }) => {
    await seedAuth(page);
    await mockApi(page, {
      "/api/sequences": [
        { id: "s-1", user_id: "u-1", name: "Холодная цепочка", is_active: true, created_at: "2026-06-01T00:00:00Z" },
      ],
      "/api/sequences/s-1": {
        sequence: { id: "s-1", user_id: "u-1", name: "Холодная цепочка", is_active: true, created_at: "2026-06-01T00:00:00Z" },
        steps: [
          { id: "st-1", sequence_id: "s-1", step_order: 1, delay_days: 0, prompt_hint: "Первое касание", body: "", channel: "email", created_at: "2026-06-01T00:00:00Z" },
        ],
      },
    });

    await page.goto("/sequences");

    await expect(page.getByRole("heading", { name: "Секвенции", exact: true })).toBeVisible();
    await expect(page.getByText("Холодная цепочка").first()).toBeVisible();
  });
});

test.describe("pipeline journey", () => {
  test("renders the pipeline board heading", async ({ page }) => {
    await seedAuth(page);
    await mockApi(page, {
      "/api/leads": [
        { id: "l-1", contact_name: "Мария Сидорова", channel: "telegram", status: "qualified", created_at: "2026-06-01T00:00:00Z", updated_at: "2026-06-01T00:00:00Z" },
      ],
    });

    await page.goto("/pipeline");

    await expect(page.getByRole("heading", { name: "Воронка продаж" })).toBeVisible();
  });
});

test.describe("settings journey", () => {
  test("renders the settings page with the channels section", async ({ page }) => {
    await seedAuth(page);
    await mockApi(page, {
      "/api/settings": {
        full_name: "Тест", email: "t@x.io", ai_provider: "ollama", ai_model: "gemma3:4b",
        auto_send: false, auto_send_delay_min: 5, aggregated_inbox_view: true,
      },
      "/api/telegram-account/status": { connected: false, phone: "" },
      "/api/onec/config": { base_url: "", auth_type: "basic", auth_secret: "", webhook_secret: "", is_active: false },
      "/api/onec/mapping": { rules: [] },
    });

    await page.goto("/settings");

    await expect(page.getByRole("heading", { name: "Настройки" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Каналы связи" })).toBeVisible();
  });
});

test.describe("analytics cost journey", () => {
  test("renders the cost analytics dashboard", async ({ page }) => {
    await seedAuth(page);
    await mockApi(page, {
      "/api/analytics/cost-ratios": {
        period: { from: "2026-06-01", to: "2026-06-30" },
        total_cost_usd: 12.34, total_calls: 42,
        leads_count: 10, qualified_leads_count: 4, converted_count: 2, drafts_sent_count: 5,
        cost_per_lead_usd: 1.23, cost_per_qualified_lead_usd: 3.08,
        cost_per_converted_usd: 6.17, cost_per_draft_sent_usd: 2.46,
      },
      "/api/audit/cost-summary": {
        total_usd: 12.34, total_calls: 42,
        by_request_type: [], by_model: [],
        period: { from: "2026-06-01", to: "2026-06-30" },
      },
    });

    await page.goto("/analytics/cost");

    await expect(page.getByRole("heading", { name: "Аналитика затрат" })).toBeVisible();
    // Stable label from CostSummaryCard rather than a brittle formatted figure.
    await expect(page.getByText("AI-расход за период")).toBeVisible();
  });
});
