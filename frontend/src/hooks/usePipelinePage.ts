import { useState, useEffect } from "react";
import { api, Lead } from "@/lib/api";
import { type Channel, getTimeAgo, STATUS_CONFIG, COLUMN_ORDER, type PipelineColumn } from "@/components/pipeline/constants";

export function usePipelinePage() {
  const [activeChannel, setActiveChannel] = useState<Channel>("all");
  const [leads, setLeads] = useState<Lead[]>([]);
  const [qualifications, setQualifications] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchLeads = () => {
      api.getLeads().then((data) => {
        setLeads(data);
        data.forEach((l) => {
          api.getQualification(l.id).then((q) => {
            if (q?.identified_need) setQualifications((prev) => ({ ...prev, [l.id]: q.identified_need }));
          }).catch(() => {});
        });
      }).catch(() => {}).finally(() => setLoading(false));
    };
    fetchLeads();
    const interval = setInterval(fetchLeads, 5000);
    return () => clearInterval(interval);
  }, []);

  const columnCounts: Record<string, number> = {};
  const columns: PipelineColumn[] = COLUMN_ORDER.map((status) => {
    const config = STATUS_CONFIG[status];
    const statusLeads = leads.filter((l) => l.status === status);
    columnCounts[config.key] = statusLeads.length;
    return {
      ...config, count: statusLeads.length,
      leads: statusLeads.map((l) => ({
        id: l.id, name: l.contact_name, company: l.company || "",
        channel: l.channel,
        preview: qualifications[l.id] || (l.first_message === "/start" ? undefined : l.first_message) || undefined,
        timeAgo: getTimeAgo(l.created_at),
      })),
    };
  });

  const totalActive = leads.filter((l) => l.status !== "closed").length;
  const filteredColumns = columns.map((col) =>
    activeChannel === "all" ? col : { ...col, leads: col.leads.filter((l) => l.channel === activeChannel) }
  );

  return {
    activeChannel,
    setActiveChannel,
    loading,
    totalActive,
    columnCounts,
    filteredColumns,
  };
}
