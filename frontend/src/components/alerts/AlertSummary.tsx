import { Lightbulb } from "lucide-react";

interface AlertSummaryProps {
  followupCount: number;
  criticalCount: number;
  warningCount: number;
}

export function AlertSummary({ followupCount, criticalCount, warningCount }: AlertSummaryProps) {
  return (
    <div className="space-y-8">
      {/* Alert Summary */}
      <div className="rounded-xl bg-[#eff4ff] p-6">
        <h4 className="mb-6 text-sm font-bold uppercase tracking-widest text-[#0d1c2e]">
          Сводка алертов
        </h4>
        <div className="space-y-6">
          <div>
            <div className="mb-2 flex items-center justify-between">
              <span className="text-sm font-medium text-[#434655]">
                Критические (4д+)
              </span>
              <span className="font-extrabold text-[#ba1a1a]">
                {criticalCount}
              </span>
            </div>
            <div className="h-1 overflow-hidden rounded-full bg-[#dce9ff]">
              <div
                className="h-full bg-[#ba1a1a]"
                style={{
                  width:
                    followupCount > 0
                      ? `${(criticalCount / followupCount) * 100}%`
                      : "0%",
                }}
              />
            </div>
          </div>
          <div>
            <div className="mb-2 flex items-center justify-between">
              <span className="text-sm font-medium text-[#434655]">
                Предупреждения (2д)
              </span>
              <span className="font-extrabold text-[#004ac6]">
                {warningCount}
              </span>
            </div>
            <div className="h-1 overflow-hidden rounded-full bg-[#dce9ff]">
              <div
                className="h-full bg-[#004ac6]"
                style={{
                  width:
                    followupCount > 0
                      ? `${(warningCount / followupCount) * 100}%`
                      : "0%",
                }}
              />
            </div>
          </div>
        </div>
      </div>

      {/* Insight card */}
      <div className="rounded-xl border border-[#c3c6d7]/10 bg-white p-6 shadow-sm">
        <div className="mb-4 flex items-center gap-3">
          <div className="flex size-8 items-center justify-center rounded-lg bg-[#e1e0ff]">
            <Lightbulb className="size-4 text-[#3e3fcc]" />
          </div>
          <p className="text-sm font-bold text-[#0d1c2e]">Знаете ли вы?</p>
        </div>
        <p className="text-xs leading-relaxed text-[#434655]">
          Лиды, которым напомнили в течение 48 часов,{" "}
          <span className="font-bold text-[#0d1c2e]">
            в 3.4 раза чаще
          </span>{" "}
          конвертируются в закрытые сделки.
        </p>
      </div>
    </div>
  );
}
