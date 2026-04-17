import { useState } from "react";
import { api } from "@/lib/api";
import { SourceCombobox } from "@/components/ui/source-combobox";

interface AddProspectFormProps {
  onAdded: () => void;
}

export function AddProspectForm({ onAdded }: AddProspectFormProps) {
  const [name, setName] = useState("");
  const [company, setCompany] = useState("");
  const [position, setPosition] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [tgUsername, setTgUsername] = useState("");
  const [whatsApp, setWhatsApp] = useState("");
  const [sourceId, setSourceId] = useState<string | null>(null);

  const handleSubmit = async () => {
    if (!name) { alert("Введите имя"); return; }
    try {
      await api.createProspect({
        name, company, title: position, email,
        phone: phone || undefined,
        telegram_username: tgUsername || undefined,
        whatsapp: whatsApp || undefined,
        source_id: sourceId || undefined,
      });
      onAdded();
      setName(""); setCompany(""); setPosition(""); setEmail("");
      setPhone(""); setTgUsername(""); setWhatsApp(""); setSourceId(null);
    } catch { alert("Ошибка добавления"); }
  };

  const fields = [
    { label: "Имя", placeholder: "Введите имя", value: name, onChange: setName },
    { label: "Компания", placeholder: "Название компании", value: company, onChange: setCompany },
    { label: "Должность", placeholder: "Напр. Head of Sales", value: position, onChange: setPosition },
    { label: "Email", placeholder: "email@example.com", value: email, onChange: setEmail, type: "email" },
    { label: "Телефон", placeholder: "+7 900 123-45-67", value: phone, onChange: setPhone, type: "tel" },
    { label: "Telegram", placeholder: "username (без @)", value: tgUsername, onChange: (v: string) => setTgUsername(v.replace("@", "")) },
    { label: "WhatsApp", placeholder: "+7 900 123-45-67", value: whatsApp, onChange: setWhatsApp },
  ];

  return (
    <div id="add-form" className="rounded-xl border border-[#c3c6d7]/10 bg-white p-6 shadow-sm">
      <h3 className="mb-6 text-xl font-bold text-[#0d1c2e]">Новый контакт</h3>
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
        {fields.map((f) => (
          <div key={f.label}>
            <label className="mb-2 block text-xs font-bold uppercase tracking-wider text-[#434655]">{f.label}</label>
            <input
              className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-2.5 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
              placeholder={f.placeholder} type={f.type || "text"} value={f.value}
              onChange={(e) => f.onChange(e.target.value)}
            />
          </div>
        ))}
        <div>
          <label className="mb-2 block text-xs font-bold uppercase tracking-wider text-[#434655]">Источник</label>
          <SourceCombobox value={sourceId} onChange={setSourceId} />
        </div>
        <button type="submit" className="mt-4 w-full rounded-lg bg-[#004ac6] py-3 font-bold text-white shadow-lg shadow-[#004ac6]/20 transition-all hover:scale-[0.98]">
          Добавить
        </button>
      </form>
    </div>
  );
}
