import { useState } from "react";

interface EditNameDialogProps {
  currentName: string;
  onSave: (name: string) => void;
  onCancel: () => void;
}

export function EditNameDialog({ currentName, onSave, onCancel }: EditNameDialogProps) {
  const [name, setName] = useState(currentName);

  const handleSave = () => {
    if (name.trim()) onSave(name.trim());
  };

  return (
    <>
      <div className="fixed inset-0 z-40 bg-black/20 backdrop-blur-sm" onClick={onCancel} />
      <div className="fixed left-1/2 top-1/2 z-50 w-96 -translate-x-1/2 -translate-y-1/2 rounded-2xl bg-white p-6 shadow-2xl">
        <h3 className="mb-4 text-lg font-bold text-[#0d1c2e]">Переименовать секвенцию</h3>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          autoFocus
          onKeyDown={(e) => {
            if (e.key === "Enter" && name.trim()) handleSave();
          }}
          className="mb-4 w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none focus:ring-2 focus:ring-[#004ac6]/20"
        />
        <div className="flex justify-end gap-3">
          <button
            onClick={onCancel}
            className="rounded-lg border border-[#c3c6d7] px-5 py-2.5 text-sm font-bold text-[#434655] hover:bg-[#eff4ff]"
          >
            Отмена
          </button>
          <button
            onClick={handleSave}
            className="rounded-lg bg-[#004ac6] px-5 py-2.5 text-sm font-bold text-white hover:brightness-110"
          >
            Сохранить
          </button>
        </div>
      </div>
    </>
  );
}
