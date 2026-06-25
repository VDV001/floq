import type { Page } from "@playwright/test";

// Token any value — the frontend only checks for its presence (the dashboard
// auth guard reads localStorage.token); the API is mocked so it's never
// validated server-side.
export async function seedAuth(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem("token", "e2e-token");
    localStorage.setItem("refresh_token", "e2e-refresh");
  });
}

// Body keyed by the longest matching URL-path substring. Each spec passes the
// responses its journey needs; sane defaults cover the endpoints every
// dashboard page hits on mount so unrelated background fetches don't 404.
export type RouteMap = Record<string, unknown>;

const DEFAULTS: RouteMap = {
  "/api/usage": { plan: "growth", limit: 1000, month_leads: 12, total_leads: 50 },
  "/api/leads/suggestion-counts": {},
  "/api/leads": [],
  "/api/prospects": [],
  "/api/sources/stats": [],
  "/api/sequences": [],
  "/api/outbound/queue": [],
  "/api/outbound/sent": [],
  "/api/outbound/stats": { draft: 0, approved: 0, sent: 0, opened: 0, replied: 0, bounced: 0 },
  "/api/settings": { auto_send: false, auto_send_delay_min: 5 },
  "/api/analytics/qualification-distribution": { step: 1, total: 0, buckets: [] },
  "/api/analytics/sequence-conversion": { steps: [] },
};

// Install a catch-all mock for **/api/**. Longest-key-first so specific paths
// (e.g. /api/outbound/stats) win over prefixes (/api/outbound). Unknown
// endpoints fall back to an empty array — harmless for list pages.
export async function mockApi(page: Page, overrides: RouteMap = {}) {
  const map: RouteMap = { ...DEFAULTS, ...overrides };
  const keys = Object.keys(map).sort((a, b) => b.length - a.length);

  await page.route("**/api/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const key = keys.find((k) => path === k || path.startsWith(k + "/") || path === k);
    const body = key !== undefined ? map[key] : [];
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(body),
    });
  });
}
