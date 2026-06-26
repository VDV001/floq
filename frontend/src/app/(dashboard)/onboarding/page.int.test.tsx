import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type { UserSettings } from "@/lib/api";

import OnboardingPage from "./page";

// Integration: real OnboardingPage + useOnboarding + lib/api.ts, network via MSW.
// On mount the hook fires four GETs (Promise.all):
//   /api/settings        -> UserSettings (drives the telegram/ai/email/imap steps)
//   /api/prospects       -> Prospect[]   (length -> "prospects" step)
//   /api/sequences       -> Sequence[]   (length -> "sequence" step)
//   /api/outbound/queue  -> OutboundMessage[] (length -> "launch" step)
function settings(over: Partial<UserSettings> = {}): UserSettings {
  return {
    full_name: "",
    email: "",
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
    auto_qualify: false,
    auto_draft: false,
    auto_send: false,
    auto_send_delay_min: 0,
    auto_followup: false,
    auto_followup_days: 0,
    auto_prospect_to_lead: false,
    auto_verify_import: false,
    aggregated_inbox_view: false,
    ...over,
  };
}

function mountWith(opts: {
  settings: UserSettings;
  prospects?: unknown[];
  sequences?: unknown[];
  outbound?: unknown[];
}) {
  server.use(
    http.get(url("/api/settings"), () => HttpResponse.json(opts.settings)),
    http.get(url("/api/prospects"), () => HttpResponse.json(opts.prospects ?? [])),
    http.get(url("/api/sequences"), () => HttpResponse.json(opts.sequences ?? [])),
    http.get(url("/api/outbound/queue"), () => HttpResponse.json(opts.outbound ?? [])),
  );
}

describe("onboarding page (integration)", () => {
  it("loads progress from the API and renders the steps with a fresh state", async () => {
    // Nothing configured -> 0 of 7 done, welcome heading.
    mountWith({ settings: settings() });

    render(<OnboardingPage />);

    // Header resolves once /api/settings lands (replaces the loading spinner).
    expect(
      await screen.findByText("Добро пожаловать в Floq"),
    ).toBeInTheDocument();
    expect(screen.getByText("0 / 7")).toBeInTheDocument();

    // All step titles render; none marked "Готово" on a fresh account.
    expect(screen.getByText("Подключите Telegram бота")).toBeInTheDocument();
    expect(screen.getByText("Настройте AI-провайдер")).toBeInTheDocument();
    expect(screen.queryByText("Готово")).not.toBeInTheDocument();
  });

  it("marks completed steps from the API responses and advances the progress count", async () => {
    // Telegram + AI configured, and one prospect imported -> 3 of 7 done.
    mountWith({
      settings: settings({ telegram_bot_active: true, ai_active: true }),
      prospects: [{ id: "p1" }],
    });

    render(<OnboardingPage />);

    expect(await screen.findByText("Добро пожаловать в Floq")).toBeInTheDocument();
    expect(screen.getByText("3 / 7")).toBeInTheDocument();

    // Three steps should now show the "Готово" badge.
    expect(screen.getAllByText("Готово")).toHaveLength(3);
  });
});
