import { useState, useEffect } from "react";
import { api, type UserSettings } from "@/lib/api";
import { STEPS, type Counts } from "@/components/onboarding/constants";

export function useOnboarding() {
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [counts, setCounts] = useState<Counts>({
    prospects: 0,
    sequences: 0,
    outbound: 0,
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      api.getSettings(),
      api.getProspects().then((p) => p.length).catch(() => 0),
      api.getSequences().then((s) => s.length).catch(() => 0),
      api.getOutboundQueue().then((q) => q.length).catch(() => 0),
    ])
      .then(([s, prospects, sequences, outbound]) => {
        setSettings(s);
        setCounts({ prospects, sequences, outbound });
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const completedSteps = settings
    ? STEPS.filter((step) => step.check(settings, counts)).length
    : 0;
  const progress = (completedSteps / STEPS.length) * 100;
  const allDone = completedSteps === STEPS.length;

  return { settings, counts, loading, completedSteps, progress, allDone };
}
