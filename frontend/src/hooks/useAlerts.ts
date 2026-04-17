import { useState, useEffect } from "react";
import { api, Lead } from "@/lib/api";
import { getSilentDays } from "@/components/alerts/helpers";

export function useAlerts() {
  const [leads, setLeads] = useState<Lead[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchLeads = () => {
      api.getLeads().then((data) => setLeads(data)).catch(() => {}).finally(() => setLoading(false));
    };
    fetchLeads();
    const interval = setInterval(fetchLeads, 5000);
    return () => clearInterval(interval);
  }, []);

  const followupLeads = leads.filter((l) => l.status === "followup");
  const featured = followupLeads[0] ?? null;
  const listAlerts = followupLeads.slice(1);

  const totalLeads = leads.length;
  const criticalCount = followupLeads.filter(
    (l) => getSilentDays(l.updated_at) >= 4
  ).length;
  const warningCount = followupLeads.length - criticalCount;

  return { loading, followupLeads, featured, listAlerts, totalLeads, criticalCount, warningCount };
}
