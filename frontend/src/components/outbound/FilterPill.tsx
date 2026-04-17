export function FilterPill<T extends string>({
  label, value, current, onChange,
}: {
  label: string; value: T; current: T; onChange: (v: T) => void;
}) {
  const active = value === current;
  return (
    <button
      onClick={() => onChange(value)}
      className={`rounded-full px-3 py-1 text-xs font-bold transition-all ${
        active ? "bg-[#004ac6] text-white" : "border border-[#c3c6d7] text-[#434655] hover:border-[#004ac6] hover:text-[#004ac6]"
      }`}
    >
      {label}
    </button>
  );
}
