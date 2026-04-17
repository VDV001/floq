import { useState } from "react";
import { Globe } from "lucide-react";
import { api } from "@/lib/api";

export function WebsiteScraper() {
  const [url, setUrl] = useState("");
  const [loading, setLoading] = useState(false);
  const [results, setResults] = useState<string[]>([]);
  const [error, setError] = useState("");

  const handleScrape = async () => {
    setLoading(true);
    setError("");
    setResults([]);
    try {
      const res = await api.scrapeWebsite(url);
      setResults(res.emails);
      if (res.emails.length === 0) setError("Email не найдены на этом сайте");
    } catch {
      setError("Не удалось загрузить сайт");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="rounded-xl border border-[#c3c6d7]/10 bg-white p-6 shadow-sm">
      <h3 className="mb-4 flex items-center gap-2 text-sm font-bold text-[#0d1c2e]">
        <Globe className="size-4" />
        Поиск email по сайту
      </h3>
      <input
        className="mb-3 w-full rounded-lg border-none bg-[#eff4ff] px-4 py-2.5 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
        placeholder="https://company.ru" value={url} onChange={(e) => setUrl(e.target.value)}
      />
      <button className="w-full rounded-lg border border-[#c3c6d7]/30 py-2.5 text-sm font-semibold text-[#0d1c2e] transition-all hover:bg-[#eff4ff] disabled:opacity-50"
        onClick={handleScrape} disabled={loading || !url}>
        {loading ? "Поиск..." : "Найти email"}
      </button>
      {results.length > 0 && (
        <div className="mt-3 space-y-1">
          <p className="text-xs font-medium text-[#434655]">Найдено: {results.length}</p>
          {results.map((email) => (
            <div key={email} className="rounded-lg bg-[#eff4ff] px-3 py-1.5 text-sm text-[#0d1c2e]">{email}</div>
          ))}
        </div>
      )}
      {error && <p className="mt-2 text-xs text-[#ba1a1a]">{error}</p>}
    </div>
  );
}
