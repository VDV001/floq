import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";

vi.mock("@/lib/api", () => ({
  api: { getLeads: vi.fn() },
}));

import { api, type Lead } from "@/lib/api";
import { useAlerts, type AlertSeverity } from "./useAlerts";

const DAY_MS = 24 * 60 * 60 * 1000;
const daysAgo = (n: number) => new Date(Date.now() - n * DAY_MS).toISOString();

function lead(over: Partial<Lead>): Lead {
  return {
    id: "l",
    user_id: "u-1",
    channel: "telegram",
    contact_name: "Контакт",
    company: "Acme",
    first_message: "Здравствуйте",
    status: "followup",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: daysAgo(1),
    ...over,
  } as Lead;
}

describe("useAlerts", () => {
  beforeEach(() => {
    vi.mocked(api.getLeads).mockReset();
  });

  // Two critical followups (silent ≥4d), one warning (silent <4d), one non-followup.
  const leads = [
    lead({ id: "crit-1", status: "followup", updated_at: daysAgo(10) }),
    lead({ id: "crit-2", status: "followup", updated_at: daysAgo(8) }),
    lead({ id: "warn-1", status: "followup", updated_at: daysAgo(1) }),
    lead({ id: "new-1", status: "new", updated_at: daysAgo(20) }),
  ];

  it("derives followups, totals and severity counts from the leads", async () => {
    vi.mocked(api.getLeads).mockResolvedValue(leads);
    const { result } = renderHook(() => useAlerts());
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.followupLeads).toHaveLength(3);
    expect(result.current.totalLeads).toBe(4);
    expect(result.current.criticalCount).toBe(2);
    expect(result.current.warningCount).toBe(1);
    // Default "all" shows every followup; featured is the first.
    expect(result.current.severity).toBe("all");
    expect(result.current.visibleFollowups).toHaveLength(3);
    expect(result.current.featured?.id).toBe("crit-1");
  });

  // One behaviour (setSeverity → visibleFollowups + featured), three variants —
  // table-driven. Summary counts must stay constant across every filter.
  const severityCases: { severity: AlertSeverity; visible: string[]; featured: string }[] = [
    { severity: "all", visible: ["crit-1", "crit-2", "warn-1"], featured: "crit-1" },
    { severity: "critical", visible: ["crit-1", "crit-2"], featured: "crit-1" },
    { severity: "warning", visible: ["warn-1"], featured: "warn-1" },
  ];

  it.each(severityCases)(
    "filters visible followups to $severity without touching the totals",
    async ({ severity, visible, featured }) => {
      vi.mocked(api.getLeads).mockResolvedValue(leads);
      const { result } = renderHook(() => useAlerts());
      await waitFor(() => expect(result.current.loading).toBe(false));

      act(() => result.current.setSeverity(severity));
      expect(result.current.visibleFollowups.map((l) => l.id)).toEqual(visible);
      expect(result.current.featured?.id).toBe(featured);
      // Summary counts always reflect every followup, regardless of the filter.
      expect(result.current.criticalCount).toBe(2);
      expect(result.current.warningCount).toBe(1);
    },
  );

  it("returns to the full list when the filter is reset to all", async () => {
    vi.mocked(api.getLeads).mockResolvedValue(leads);
    const { result } = renderHook(() => useAlerts());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => result.current.setSeverity("critical"));
    act(() => result.current.setSeverity("all"));
    expect(result.current.visibleFollowups).toHaveLength(3);
  });

  it("keeps the last good state and stops loading when the fetch fails", async () => {
    vi.mocked(api.getLeads).mockRejectedValue(new Error("network"));
    const { result } = renderHook(() => useAlerts());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.followupLeads).toHaveLength(0);
  });
});
