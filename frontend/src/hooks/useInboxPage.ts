import { useState, useEffect } from "react";
import { api } from "@/lib/api";
import { getTimeAgo } from "@/components/inbox/helpers";
import { type InboxLead, PIPELINE_STAGES_CONFIG, mapStatus } from "@/components/inbox/constants";

export function useInboxPage() {
  const [activeFilter, setActiveFilter] = useState<string>("Все");
  const [activeStage, setActiveStage] = useState("new");
  const [loading, setLoading] = useState(true);
  const [leads, setLeads] = useState<InboxLead[]>([]);
  const [statusCounts, setStatusCounts] = useState<Record<string, number>>({});
  const [sourceFilter, setSourceFilter] = useState("");
  const [suggestionCounts, setSuggestionCounts] = useState<Record<string, number>>({});

  useEffect(() => {
    const fetchLeads = () => {
      api.getLeads().then((data) => {
        const mapped: InboxLead[] = data.map((l) => ({
          id: l.id, company: l.company || "—", contact: l.contact_name,
          channel: l.channel as "email" | "telegram",
          preview: l.first_message === "/start" ? "Загрузка..." : (l.first_message || "Нет сообщений"),
          timeAgo: getTimeAgo(l.created_at), status: mapStatus(l.status), apiStatus: l.status, sourceName: l.source_name,
        }));
        setLeads(mapped);
        data.forEach((l) => {
          if (l.first_message === "/start") {
            api.getQualification(l.id).then((q) => {
              if (q?.identified_need) setLeads((prev) => prev.map((lead) => lead.id === l.id ? { ...lead, preview: q.identified_need } : lead));
            }).catch(() => {});
          }
        });
        const counts: Record<string, number> = {};
        for (const l of data) counts[l.status] = (counts[l.status] || 0) + 1;
        setStatusCounts(counts);
      }).catch(() => {}).finally(() => setLoading(false));
    };
    fetchLeads();
    const fetchSuggestionCounts = () => { api.getSuggestionCounts().then(setSuggestionCounts).catch(() => {}); };
    fetchSuggestionCounts();
    const interval = setInterval(() => { fetchLeads(); fetchSuggestionCounts(); }, 30000);
    return () => clearInterval(interval);
  }, []);

  const stageConfig = PIPELINE_STAGES_CONFIG.find((s) => s.id === activeStage);
  const filteredLeads = leads.filter((lead) => {
    if (stageConfig && lead.apiStatus !== stageConfig.apiStatus) return false;
    if (sourceFilter && lead.sourceName !== sourceFilter) return false;
    return true;
  });

  return {
    activeFilter,
    setActiveFilter,
    activeStage,
    setActiveStage,
    loading,
    leads,
    statusCounts,
    sourceFilter,
    setSourceFilter,
    suggestionCounts,
    filteredLeads,
  };
}
