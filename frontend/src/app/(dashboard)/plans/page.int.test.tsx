import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";

import PlansPage from "./page";

// Integration: real PlansPage + lib/api.ts, network via MSW.
// On mount the page fires exactly one GET:
//   /api/usage  (current plan + leads usage banner)
type Usage = { plan: string; limit: number; month_leads: number; total_leads: number };

function mountWith(usage: Usage) {
  server.use(http.get(url("/api/usage"), () => HttpResponse.json(usage)));
}

describe("plans page (integration)", () => {
  it("renders the three pricing plans with prices and features", async () => {
    mountWith({ plan: "growth", limit: 1000, month_leads: 250, total_leads: 4000 });

    render(<PlansPage />);

    // Static pricing cards.
    expect(await screen.findByRole("heading", { name: "Starter" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Growth" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Pro" })).toBeInTheDocument();
    expect(screen.getByText("3 900")).toBeInTheDocument();
    expect(screen.getByText("7 900")).toBeInTheDocument();
    expect(screen.getByText("14 900")).toBeInTheDocument();
    expect(screen.getByText("Популярный")).toBeInTheDocument();
    expect(screen.getByText("Максимум")).toBeInTheDocument();
    // A representative feature line.
    expect(screen.getByText("Безлимитные лиды")).toBeInTheDocument();
  });

  it("marks the API-reported plan as current and renders the usage banner", async () => {
    mountWith({ plan: "growth", limit: 1000, month_leads: 250, total_leads: 4000 });

    render(<PlansPage />);

    // Usage banner appears only after /api/usage resolves.
    expect(await screen.findByText(/250 \/ 1000 лидов использовано/)).toBeInTheDocument();

    // Growth is the reported plan -> its CTA is the disabled "Текущий план".
    const currentBtn = screen.getByRole("button", { name: /Текущий план/ });
    expect(currentBtn).toBeDisabled();

    // The other plans get actionable CTAs. Pro (limit Infinity > 1000) is an upgrade.
    expect(screen.getByRole("button", { name: "Перейти на Pro" })).toBeInTheDocument();
    // Starter (limit 200 <= 1000) is a "Выбрать" downgrade-style CTA.
    expect(screen.getByRole("button", { name: "Выбрать Starter" })).toBeInTheDocument();
  });

  it("falls back to the default current plan when /api/usage fails", async () => {
    server.use(
      http.get(url("/api/usage"), () => new HttpResponse(null, { status: 500 })),
    );

    render(<PlansPage />);

    // No usage banner without data; the default current plan is "growth".
    expect(await screen.findByRole("button", { name: /Текущий план/ })).toBeInTheDocument();
    expect(screen.queryByText(/лидов использовано/)).not.toBeInTheDocument();
  });
});
