import { useState } from "react";
import { Send, Pencil, X, Clock } from "lucide-react";
import { api } from "@/lib/api";
import type { UIMessage } from "./constants";

interface MessageCardProps {
  msg: UIMessage;
  isQueue: boolean;
  onApprove: (id: string) => void;
  onReject: (id: string) => void;
  onEdited: (id: string, newBody: string) => void;
}

export function MessageCard({ msg, isQueue, onApprove, onReject, onEdited }: MessageCardProps) {
  const [editing, setEditing] = useState(false);
  const [editText, setEditText] = useState("");

  const handleEdit = () => { setEditing(true); setEditText(msg.body); };
  const handleSave = async () => {
    try {
      await api.editMessage(msg.id, editText);
      onEdited(msg.id, editText);
      setEditing(false);
    } catch { /* ignore */ }
  };
  const handleCancel = () => { setEditing(false); setEditText(""); };

  return (
    <div className="flex flex-col items-start gap-6 rounded-2xl border border-transparent bg-white p-6 transition-all duration-300 hover:border-[#004ac6]/10 hover:shadow-xl hover:shadow-blue-900/5 lg:flex-row lg:items-center">
      <div className="flex w-full flex-shrink-0 items-center gap-4 lg:w-64">
        <div className={`flex size-12 shrink-0 items-center justify-center rounded-full text-lg font-bold ${msg.avatarBg} text-[#0d1c2e]`}>
          {msg.initials}
        </div>
        <div className="min-w-0">
          <h4 className="truncate font-bold text-[#0d1c2e]">{msg.name}</h4>
          <p className="truncate text-xs text-[#434655]">{msg.role}</p>
        </div>
      </div>

      <div className="min-w-0 flex-1">
        <div className="mb-2 flex items-center gap-3">
          <span className="rounded bg-[#e1e0ff] px-2 py-0.5 text-[10px] font-black uppercase text-[#2f2ebe]">{msg.step}</span>
          <span className="text-[11px] font-bold uppercase tracking-tight text-[#737686]">Sequence: {msg.sequence}</span>
          <span className={`rounded-full px-2 py-0.5 text-[10px] font-bold uppercase ${
            msg.channel === "email" ? "bg-blue-50 text-blue-600" : msg.channel === "telegram" ? "bg-sky-50 text-sky-600" : "bg-amber-50 text-amber-600"
          }`}>
            {msg.channel === "email" ? "Email" : msg.channel === "telegram" ? "Telegram" : "Звонок"}
          </span>
        </div>
        {editing ? (
          <div className="flex flex-col gap-2">
            <textarea value={editText} onChange={(e) => setEditText(e.target.value)} rows={4}
              className="w-full rounded-xl border border-[#c3c6d7] bg-white p-3 text-sm leading-relaxed text-[#434655] outline-none focus:border-[#004ac6] focus:ring-2 focus:ring-[#004ac6]/20" />
            <div className="flex gap-2">
              <button onClick={handleSave} className="rounded-lg bg-[#004ac6] px-3 py-1.5 text-xs font-bold text-white hover:opacity-90">Сохранить</button>
              <button onClick={handleCancel} className="rounded-lg border border-[#c3c6d7] px-3 py-1.5 text-xs font-bold text-[#434655] hover:bg-[#eff4ff]">Отмена</button>
            </div>
          </div>
        ) : (
          <p className="line-clamp-2 text-sm italic leading-relaxed text-[#434655]">&ldquo;{msg.body}&rdquo;</p>
        )}
        <div className="mt-2 flex items-center gap-2 text-[11px] font-bold uppercase text-[#004ac6]/60">
          <Clock className="size-3.5" />
          Запланировано: {msg.scheduledAt}
        </div>
      </div>

      {isQueue ? (
        <div className="flex w-full items-center gap-2 lg:w-auto">
          <button onClick={() => onApprove(msg.id)}
            className="flex flex-1 items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] px-4 py-2.5 text-xs font-bold text-white transition-opacity hover:opacity-90 lg:flex-none">
            <Send className="size-3.5" /> Подтвердить
          </button>
          <button onClick={handleEdit}
            className="flex size-10 items-center justify-center rounded-xl border border-[#c3c6d7] text-[#434655] transition-colors hover:bg-[#eff4ff]">
            <Pencil className="size-[18px]" />
          </button>
          <button onClick={() => onReject(msg.id)}
            className="flex size-10 items-center justify-center rounded-xl text-[#ba1a1a] transition-colors hover:bg-[#ffdad6]/20">
            <X className="size-[18px]" />
          </button>
        </div>
      ) : (
        <div className="flex shrink-0 items-center gap-2">
          <span className={`rounded-full px-3 py-1 text-[10px] font-bold uppercase tracking-wider ${
            msg.status === "sent" ? "bg-green-100 text-green-700" : msg.status === "rejected" ? "bg-red-100 text-red-600" : "bg-blue-100 text-blue-700"
          }`}>
            {msg.status === "sent" ? "Отправлено" : msg.status === "rejected" ? "Отклонено" : "Одобрено"}
          </span>
        </div>
      )}
    </div>
  );
}
