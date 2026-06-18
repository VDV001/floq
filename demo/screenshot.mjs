// Floq demo — headless-обход приложения и снятие скриншотов сценариев.
// Playwright берётся из внешней папки, чтобы не тянуть node_modules в репозиторий.
// Запуск:  node demo/screenshot.mjs   (фронт на :3000, бэкенд на :8080 должны быть подняты)
import { createRequire } from "module";
const require = createRequire("/Users/daniil/floq-demo/");
const { chromium } = require("playwright");

const BASE = "http://localhost:3000";
const EMAIL = "demo@floq.app";
const PASSWORD = "demo123";
const OUT = new URL("./screens/", import.meta.url).pathname;

// Конкретные лиды из seed для карточек
const LEAD_QUALIFIED = "b1111111-0000-0000-0000-000000000001"; // Алексей Смирнов, score 82
const LEAD_DRAFT = "b1111111-0000-0000-0000-000000000002"; // Марина Котова, AI-черновик

// [файл, путь, fullPage?]
const SHOTS = [
  ["01-login", "/login", false],
  ["02-inbox", "/inbox", false],
  ["03-lead-qualification", `/inbox/${LEAD_QUALIFIED}`, true],
  ["04-lead-draft", `/inbox/${LEAD_DRAFT}`, true],
  ["05-pending-replies", "/inbox/pending", false],
  ["06-leads", "/alerts", false],
  ["07-pipeline", "/pipeline", false],
  ["08-automations", "/automations", true],
  ["09-prospects", "/prospects", false],
  ["10-sequences", "/sequences", false],
  ["11-outbound", "/outbound", false],
  ["12-analytics-sequences", "/analytics/sequences", true],
  ["13-analytics-cost", "/analytics/cost", true],
  ["14-settings", "/settings", true],
  ["15-plans", "/plans", false],
];

const run = async () => {
  const browser = await chromium.launch();
  const ctx = await browser.newContext({
    viewport: { width: 1440, height: 1600 },
    deviceScaleFactor: 2,
    locale: "ru-RU",
    colorScheme: "light",
  });
  const page = await ctx.newPage();

  // --- логин один раз ---
  console.log("login…");
  await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });
  // первый кадр — экран логина (до ввода)
  await page.screenshot({ path: `${OUT}01-login.png` });
  await page.locator('input[type="email"]').fill(EMAIL);
  await page.locator('input[type="password"]').fill(PASSWORD);
  await page.locator('button[type="submit"]').click();
  await page.waitForURL("**/inbox", { timeout: 15000 });
  await page.waitForTimeout(1500);
  console.log("logged in");

  // --- остальные кадры ---
  for (const [name, path, full] of SHOTS) {
    if (name === "01-login") continue; // уже сняли
    try {
      await page.goto(`${BASE}${path}`, { waitUntil: "networkidle", timeout: 20000 });
      await page.waitForTimeout(2000); // дать SWR/анимациям осесть
      await page.screenshot({ path: `${OUT}${name}.png`, fullPage: !!full });
      console.log(`✓ ${name}  (${path})`);
    } catch (e) {
      console.log(`✗ ${name}  (${path}) — ${e.message}`);
      // всё равно снимем что есть
      await page.screenshot({ path: `${OUT}${name}.png`, fullPage: !!full }).catch(() => {});
    }
  }

  await browser.close();
  console.log("done →", OUT);
};

run().catch((e) => {
  console.error(e);
  process.exit(1);
});
