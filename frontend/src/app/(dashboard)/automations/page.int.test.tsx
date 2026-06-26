import { describe, it, expect } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type { UserSettings } from "@/lib/api";

import AutomationsPage from "./page";

// Integration: real AutomationsPage + useAutomations + lib/api.ts, network via MSW.
// On mount the page fires exactly one GET (/api/settings) to hydrate the toggles
// and inputs. Toggling a switch debounces a PUT (/api/settings) after 500ms.
const settings: UserSettings = {
  full_name: "Тест",
  email: "test@floq.io",
  telegram_bot_token: "",
  telegram_bot_active: false,
  imap_host: "",
  imap_port: "",
  imap_user: "",
  imap_password: "",
  resend_api_key: "",
  smtp_host: "",
  smtp_port: "",
  smtp_user: "",
  smtp_password: "",
  smtp_active: false,
  ai_provider: "",
  ai_model: "",
  ai_api_key: "",
  imap_active: false,
  resend_active: false,
  ai_active: false,
  notify_telegram: false,
  notify_email_digest: false,
  auto_qualify: true,
  auto_draft: true,
  auto_send: false,
  auto_send_delay_min: 5,
  auto_followup: true,
  auto_followup_days: 2,
  auto_prospect_to_lead: true,
  auto_verify_import: false,
  aggregated_inbox_view: false,
};

function mountWith(
  overrides: Partial<UserSettings> = {},
  extra: Parameters<typeof server.use> = [],
) {
  server.use(
    http.get(url("/api/settings"), () => HttpResponse.json({ ...settings, ...overrides })),
    ...extra,
  );
}

describe("automations page (integration)", () => {
  it("hydrates the toggles from the settings API on mount", async () => {
    // Card titles render from constants, switches reflect the fetched settings.
    mountWith();

    render(<AutomationsPage />);

    expect(await screen.findByText("Авто-квалификация")).toBeInTheDocument();
    expect(screen.getByText("Авто-черновик")).toBeInTheDocument();
    expect(screen.getByText("Верификация при импорте")).toBeInTheDocument();

    // 4 of 6 automations are on in the fixture -> the QuickActions banner reports it.
    await waitFor(() =>
      expect(
        screen.getByText(
          "Включено 4 из 6 автоматизаций. Включите все для максимальной эффективности.",
        ),
      ).toBeInTheDocument(),
    );

    // 6 automation switches.
    expect(screen.getAllByRole("switch")).toHaveLength(6);
  });

  it("persists a toggle change through a debounced PUT to the settings API", async () => {
    const user = userEvent.setup({ delay: null });
    let putBody: Partial<UserSettings> | null = null;
    mountWith({}, [
      http.put(url("/api/settings"), async ({ request }) => {
        putBody = (await request.json()) as Partial<UserSettings>;
        return HttpResponse.json({ ...settings, auto_qualify: false });
      }),
    ]);

    render(<AutomationsPage />);
    await screen.findByText("Авто-квалификация");

    // First card is "Авто-квалификация" (default on) -> flip it off.
    const firstSwitch = screen.getAllByRole("switch")[0];
    await user.click(firstSwitch);

    // The hook debounces the save by 500ms; wait for the PUT to fire.
    await waitFor(() => expect(putBody).not.toBeNull());

    // The payload maps every toggle/input to its settings field.
    expect(putBody).toMatchObject({
      auto_qualify: false,
      auto_draft: true,
      auto_send: false,
      auto_followup: true,
      auto_prospect_to_lead: true,
      auto_verify_import: false,
      auto_send_delay_min: 5,
      auto_followup_days: 2,
    });

    // Banner now reflects 3 enabled automations.
    await waitFor(() =>
      expect(
        screen.getByText(
          "Включено 3 из 6 автоматизаций. Включите все для максимальной эффективности.",
        ),
      ).toBeInTheDocument(),
    );
  });

  it("enables every automation at once via the QuickActions button", async () => {
    const user = userEvent.setup({ delay: null });
    let putBody: Partial<UserSettings> | null = null;
    mountWith({}, [
      http.put(url("/api/settings"), async ({ request }) => {
        putBody = (await request.json()) as Partial<UserSettings>;
        return HttpResponse.json(settings);
      }),
    ]);

    render(<AutomationsPage />);
    await screen.findByText("Авто-квалификация");

    // Fixture has 4/6 on -> button reads "Включить все".
    await user.click(screen.getByRole("button", { name: "Включить все" }));

    // All six switches flip on and the banner reports the max state.
    await waitFor(() =>
      expect(
        screen.getByText("Все автоматизации включены. Система работает на максимум."),
      ).toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: "Выключить все" })).toBeInTheDocument();

    // The debounced PUT carries every toggle as enabled.
    await waitFor(() => expect(putBody).not.toBeNull());
    expect(putBody).toMatchObject({
      auto_qualify: true,
      auto_draft: true,
      auto_send: true,
      auto_followup: true,
      auto_prospect_to_lead: true,
      auto_verify_import: true,
    });
  });

  it("persists a delay input change through the settings API", async () => {
    let putBody: Partial<UserSettings> | null = null;
    mountWith({}, [
      http.put(url("/api/settings"), async ({ request }) => {
        putBody = (await request.json()) as Partial<UserSettings>;
        return HttpResponse.json(settings);
      }),
    ]);

    render(<AutomationsPage />);
    await screen.findByText("Авто-квалификация");

    // Two numeric inputs: [auto-send delay (5), auto-followup days (2)].
    const [delayInput] = screen.getAllByRole("spinbutton");
    fireEvent.change(delayInput, { target: { value: "15" } });

    await waitFor(() => expect(putBody).not.toBeNull());
    expect(putBody).toMatchObject({
      auto_send_delay_min: 15,
      auto_followup_days: 2,
    });
  });

  it("keeps the optimistic toggle state when the save request fails", async () => {
    const user = userEvent.setup({ delay: null });
    let putFired = false;
    mountWith({}, [
      http.put(url("/api/settings"), () => {
        putFired = true;
        return HttpResponse.json({ error: "boom" }, { status: 500 });
      }),
    ]);

    render(<AutomationsPage />);
    await screen.findByText("Авто-квалификация");

    // auto-send (3rd switch) starts off; flip it on -> 4 -> 5 enabled.
    const autoSend = screen.getAllByRole("switch")[2];
    await user.click(autoSend);

    // The failed PUT is swallowed; the optimistic banner still updates.
    await waitFor(() =>
      expect(
        screen.getByText(
          "Включено 5 из 6 автоматизаций. Включите все для максимальной эффективности.",
        ),
      ).toBeInTheDocument(),
    );
    await waitFor(() => expect(putFired).toBe(true));
  });
});
