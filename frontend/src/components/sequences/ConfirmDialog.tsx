interface ConfirmDialogProps {
  title: string;
  message: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmDialog({ title, message, onConfirm, onCancel }: ConfirmDialogProps) {
  return (
    <>
      <div className="fixed inset-0 z-40 bg-black/20 backdrop-blur-sm" onClick={onCancel} />
      <div className="fixed left-1/2 top-1/2 z-50 w-96 -translate-x-1/2 -translate-y-1/2 rounded-2xl bg-white p-6 shadow-2xl">
        <h3 className="mb-2 text-lg font-bold text-[#0d1c2e]">{title}</h3>
        <p className="mb-6 text-sm text-[#434655]">{message}</p>
        <div className="flex justify-end gap-3">
          <button
            onClick={onCancel}
            className="rounded-lg border border-[#c3c6d7] px-5 py-2.5 text-sm font-bold text-[#434655] hover:bg-[#eff4ff]"
          >
            Отмена
          </button>
          <button
            onClick={onConfirm}
            className="rounded-lg bg-red-500 px-5 py-2.5 text-sm font-bold text-white hover:bg-red-600"
          >
            Удалить
          </button>
        </div>
      </div>
    </>
  );
}
