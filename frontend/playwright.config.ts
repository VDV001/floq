import { defineConfig, devices } from "@playwright/test";

// E2E config. The backend is mocked at the network layer (page.route on
// **/api/**), so these tests need only the Next.js dev server — no Postgres,
// Redis or Go backend. This keeps the suite deterministic and locally runnable
// while still exercising the real browser, real Next build, routing and
// hydration (the value e2e adds over the jsdom integration suite).
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? "github" : "list",
  timeout: 90_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: "http://localhost:3000",
    navigationTimeout: 60_000,
    trace: "on-first-retry",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: {
    command: "npm run dev",
    url: "http://localhost:3000",
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
  },
});
