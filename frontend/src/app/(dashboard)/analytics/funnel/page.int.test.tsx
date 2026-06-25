import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type {
  QualificationDistributionResponse,
  SequenceConversionResponse,
} from "@/lib/api";
import FunnelAnalyticsPage from "./page";

// AnalyticsTabs reads usePathname; pin it so the page mounts without a Next
// router context (same pattern as the other analytics integration tests).
vi.mock("next/navigation", () => ({
  usePathname: () => "/analytics/funnel",
}));

function mountWith(
  distribution: QualificationDistributionResponse,
  conversion: SequenceConversionResponse,
) {
  server.use(
    http.get(url("/api/analytics/qualification-distribution"), () => HttpResponse.json(distribution)),
    http.get(url("/api/analytics/sequence-conversion"), () => HttpResponse.json(conversion)),
  );
}

describe("funnel analytics page (integration)", () => {
  it("loads the distribution and conversion from the API and renders them", async () => {
    mountWith(
      {
        step: 10,
        total: 3,
        buckets: [
          { lo: 0, hi: 9, label: "0–9", count: 1 },
          { lo: 80, hi: 89, label: "80–89", count: 2 },
        ],
      },
      {
        steps: [
          {
            sequence_id: "s-1",
            sequence_name: "Warm intro",
            step_order: 1,
            entered: 10,
            replied: 4,
            advanced: 2,
            reply_rate: 0.4,
            advance_rate: 0.2,
          },
        ],
      },
    );

    render(<FunnelAnalyticsPage />);

    // Distribution card
    expect(await screen.findByText("Распределение скоров квалификации")).toBeInTheDocument();
    expect(screen.getByText("всего: 3")).toBeInTheDocument();
    expect(screen.getByText("80–89")).toBeInTheDocument();

    // Conversion table
    expect(screen.getByText("Конверсия по шагам секвенций")).toBeInTheDocument();
    expect(screen.getByText("Warm intro")).toBeInTheDocument();
    expect(screen.getByText("40%")).toBeInTheDocument(); // reply_rate
    expect(screen.getByText("20%")).toBeInTheDocument(); // advance_rate
  });

  it("shows empty states when there is no funnel data", async () => {
    mountWith({ step: 10, total: 0, buckets: [] }, { steps: [] });

    render(<FunnelAnalyticsPage />);

    expect(await screen.findByText("Пока нет квалифицированных лидов.")).toBeInTheDocument();
    expect(screen.getByText("Пока нет отправленных шагов.")).toBeInTheDocument();
  });

  it("surfaces an error when the API fails", async () => {
    server.use(
      http.get(url("/api/analytics/qualification-distribution"), () => new HttpResponse(null, { status: 500 })),
      http.get(url("/api/analytics/sequence-conversion"), () => HttpResponse.json({ steps: [] })),
    );

    render(<FunnelAnalyticsPage />);

    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });
});
