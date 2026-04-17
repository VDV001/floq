import { Check, X } from "lucide-react";

type TestResult = { success: boolean; message?: string; error?: string } | null;

export function StatusBanner({ result, onDismiss }: { result: TestResult; onDismiss: () => void }) {
  if (!result) return null;
  return (
    <div
      className={`mt-3 flex items-center justify-between rounded-lg px-4 py-2.5 text-sm font-medium ${
        result.success
          ? "bg-green-50 text-green-700 ring-1 ring-green-200"
          : "bg-red-50 text-red-600 ring-1 ring-red-200"
      }`}
    >
      <span className="flex items-center gap-2">
        {result.success ? <Check className="size-4" /> : <X className="size-4" />}
        {result.success ? result.message : result.error}
      </span>
      <button onClick={onDismiss} className="ml-2 opacity-60 hover:opacity-100">
        <X className="size-3.5" />
      </button>
    </div>
  );
}
