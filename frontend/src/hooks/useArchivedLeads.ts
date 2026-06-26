"use client";

import { useEffect, useState } from "react";
import { api, Lead } from "@/lib/api";

// useArchivedLeads loads the archive-view feed (only archived leads, newest
// first). It owns the list so the page can drop a row optimistically the
// moment an unarchive succeeds — an unarchived lead belongs back in the
// working inbox, not in this view.
export function useArchivedLeads() {
  const [loading, setLoading] = useState(true);
  const [leads, setLeads] = useState<Lead[]>([]);

  useEffect(() => {
    let cancelled = false;
    api
      .getArchivedLeads()
      .then((data) => {
        if (!cancelled) setLeads(data);
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // removeLead drops a lead from the local list after it has been unarchived.
  const removeLead = (id: string) =>
    setLeads((prev) => prev.filter((l) => l.id !== id));

  return { loading, leads, removeLead };
}
