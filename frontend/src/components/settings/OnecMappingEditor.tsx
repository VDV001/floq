import { Plus, Trash2, Loader2, Save } from "lucide-react";
import type { OnecEventKind, OnecMappingRule } from "@/lib/api";
import { StatusBanner } from "./StatusBanner";

type Result = { success: boolean; message?: string; error?: string } | null;

interface OnecMappingEditorProps {
  rules: OnecMappingRule[];
  addRule: () => void;
  updateRule: (index: number, patch: Partial<OnecMappingRule>) => void;
  removeRule: (index: number) => void;
  saving: boolean;
  result: Result;
  setResult: (v: Result) => void;
  onSave: () => void;
}

// KIND_OPTIONS is the closed set of Floq event kinds a 1C document type can map
// onto — mirrors the backend EventKind enum.
const KIND_OPTIONS: { value: OnecEventKind; label: string }[] = [
  { value: "payment", label: "Оплата" },
  { value: "counterparty_created", label: "Новый контрагент" },
  { value: "order_status", label: "Статус заказа" },
  { value: "shipment", label: "Отгрузка" },
];

const cell = "rounded-md border-none bg-[#eff4ff] px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-[#004ac6]/20";

export function OnecMappingEditor({ rules, addRule, updateRule, removeRule, saving, result, setResult, onSave }: OnecMappingEditorProps) {
  return (
    <section className="rounded-xl bg-white p-8 shadow-sm ring-1 ring-[#c3c6d7]/10">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h3 className="text-xl font-bold text-[#0d1c2e]">Маппинг событий 1С</h3>
          <p className="mt-1 text-sm text-[#434655]">
            Сопоставьте типы объектов вашей конфигурации 1С с каноническими событиями Floq.
          </p>
        </div>
        <button onClick={addRule}
          className="flex items-center gap-1.5 rounded-lg bg-[#eff4ff] px-3 py-2 text-sm font-bold text-[#0d1c2e] transition-colors hover:bg-[#dce9ff]">
          <Plus className="size-4" /> Добавить правило
        </button>
      </div>

      {rules.length === 0 ? (
        <p className="rounded-lg bg-[#eff4ff] px-4 py-6 text-center text-sm text-[#434655]">
          Нет правил маппинга. Добавьте первое правило, чтобы события 1С начали попадать в Floq.
        </p>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-xs font-bold uppercase tracking-wide text-[#434655]">
                <th className="px-2 py-1">Тип объекта 1С</th>
                <th className="px-2 py-1">Событие Floq</th>
                <th className="px-2 py-1">Поле email</th>
                <th className="px-2 py-1">Поле имени</th>
                <th className="px-2 py-1">Поле компании</th>
                <th className="px-2 py-1" />
              </tr>
            </thead>
            <tbody>
              {rules.map((r, i) => (
                <tr key={i}>
                  <td className="px-2 py-1">
                    <input type="text" aria-label={`Тип объекта 1С, правило ${i + 1}`} placeholder="Документ.ОплатаПокупателя"
                      value={r.external_type} onChange={(e) => updateRule(i, { external_type: e.target.value })} className={cell} />
                  </td>
                  <td className="px-2 py-1">
                    <select aria-label={`Событие Floq, правило ${i + 1}`} value={r.kind}
                      onChange={(e) => updateRule(i, { kind: e.target.value as OnecEventKind })} className={cell}>
                      {KIND_OPTIONS.map((k) => (
                        <option key={k.value} value={k.value}>{k.label}</option>
                      ))}
                    </select>
                  </td>
                  <td className="px-2 py-1">
                    <input type="text" aria-label={`Поле email, правило ${i + 1}`} placeholder="email"
                      value={r.email_field} onChange={(e) => updateRule(i, { email_field: e.target.value })} className={cell} />
                  </td>
                  <td className="px-2 py-1">
                    <input type="text" aria-label={`Поле имени, правило ${i + 1}`} placeholder="name"
                      value={r.name_field ?? ""} onChange={(e) => updateRule(i, { name_field: e.target.value })} className={cell} />
                  </td>
                  <td className="px-2 py-1">
                    <input type="text" aria-label={`Поле компании, правило ${i + 1}`} placeholder="company"
                      value={r.company_field ?? ""} onChange={(e) => updateRule(i, { company_field: e.target.value })} className={cell} />
                  </td>
                  <td className="px-2 py-1">
                    <button onClick={() => removeRule(i)} aria-label={`Удалить правило ${i + 1}`}
                      className="rounded-md p-2 text-[#ba1a1a] transition-colors hover:bg-[#ba1a1a]/10">
                      <Trash2 className="size-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <button onClick={onSave} disabled={saving}
        className="mt-6 flex w-full items-center justify-center gap-2 rounded-lg bg-[#004ac6] py-3 text-sm font-bold text-white transition-colors hover:bg-[#0039a6] disabled:opacity-50">
        {saving ? <Loader2 className="size-[18px] animate-spin" /> : <Save className="size-[18px]" />}
        {saving ? "Сохраняем..." : "Сохранить маппинг"}
      </button>
      <StatusBanner result={result ? { ...result, message: result.success ? "Маппинг сохранён" : undefined } : null} onDismiss={() => setResult(null)} />
    </section>
  );
}
