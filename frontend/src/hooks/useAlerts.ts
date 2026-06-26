import { useState, useEffect } from "react";
import { api, Lead } from "@/lib/api";
import { getSilentDays } from "@/components/alerts/helpers";

// A lead is "critical" once it has been silent for ≥4 days — the same
// threshold the summary card uses to split critical vs warning.
const CRITICAL_SILENT_DAYS = 4;
const isCritical = (l: Lead) => getSilentDays(l.updated_at) >= CRITICAL_SILENT_DAYS;

export type AlertSeverity = "all" | "critical" | "warning";

export function useAlerts() {
  const [leads, setLeads] = useState<Lead[]>([]);
  const [loading, setLoading] = useState(true);
  const [severity, setSeverity] = useState<AlertSeverity>("all");

  useEffect(() => {
    const fetchLeads = () => {
      api.getLeads().then((data) => setLeads(data)).catch(() => {}).finally(() => setLoading(false));
    };
    fetchLeads();
    const interval = setInterval(fetchLeads, 5000);
    return () => clearInterval(interval);
  }, []);

  const followupLeads = leads.filter((l) => l.status === "followup");

  // The severity filter narrows what the operator sees (featured + list)
  // without touching the totals/summary, which always reflect every followup.
  const visibleFollowups =
    severity === "critical"
      ? followupLeads.filter(isCritical)
      : severity === "warning"
        ? followupLeads.filter((l) => !isCritical(l))
        : followupLeads;

  const featured = visibleFollowups[0] ?? null;
  const listAlerts = visibleFollowups.slice(1);

  const totalLeads = leads.length;
  const criticalCount = followupLeads.filter(isCritical).length;
  const warningCount = followupLeads.length - criticalCount;

  return {
    loading,
    severity,
    setSeverity,
    followupLeads,
    visibleFollowups,
    featured,
    listAlerts,
    totalLeads,
    criticalCount,
    warningCount,
  };
}
