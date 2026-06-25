"use client";

import type { SequenceConversionResponse } from "@/lib/api";

interface SequenceConversionTableProps {
  data: SequenceConversionResponse;
}

function pct(rate: number): string {
  return `${Math.round(rate * 100)}%`;
}

// SequenceConversionTable renders the per-(sequence, step) funnel: how many
// prospects entered each step, replied, and advanced to the next, with the
// derived rates. Rows are already ordered by sequence then step by the API.
export function SequenceConversionTable({ data }: SequenceConversionTableProps) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white">
      <h2 className="px-4 py-2 text-sm font-semibold text-slate-700 border-b border-slate-100">
        Конверсия по шагам секвенций
      </h2>
      {data.steps.length === 0 ? (
        <p className="p-6 text-center text-sm text-slate-500">Пока нет отправленных шагов.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50">
              <tr>
                <th scope="col" className="px-4 py-2 text-left font-medium text-slate-700">Секвенция</th>
                <th scope="col" className="px-4 py-2 text-right font-medium text-slate-700">Шаг</th>
                <th scope="col" className="px-4 py-2 text-right font-medium text-slate-700">Дошли</th>
                <th scope="col" className="px-4 py-2 text-right font-medium text-slate-700">Ответили</th>
                <th scope="col" className="px-4 py-2 text-right font-medium text-slate-700">Перешли дальше</th>
                <th scope="col" className="px-4 py-2 text-right font-medium text-slate-700">% ответов</th>
                <th scope="col" className="px-4 py-2 text-right font-medium text-slate-700">% перехода</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {data.steps.map((s) => (
                <tr key={`${s.sequence_id}-${s.step_order}`}>
                  <td className="px-4 py-2 font-medium text-slate-900">{s.sequence_name}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{s.step_order}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{s.entered}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{s.replied}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{s.advanced}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{pct(s.reply_rate)}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{pct(s.advance_rate)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
