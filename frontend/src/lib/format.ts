export function getTimeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "Только что";
  if (mins < 60) return `${mins} мин`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours} ч`;
  const days = Math.floor(hours / 24);
  return `${days} д`;
}

export function getInitials(name: string): string {
  return name.split(" ").map((w) => w[0]).join("").slice(0, 2).toUpperCase();
}

// pluralRu picks the right Russian noun form for the given count. Uses
// Intl.PluralRules so we don't hand-roll the mod-100/mod-10 logic per
// caller. Categories: "one" (1, 21, 31, …), "few" (2-4, 22-24, …),
// "many" (everything else: 0, 5-20, 25-30, …).
//
//   pluralRu(1, "ошибка", "ошибки", "ошибок") -> "ошибка"
//   pluralRu(2, "ошибка", "ошибки", "ошибок") -> "ошибки"
//   pluralRu(5, "ошибка", "ошибки", "ошибок") -> "ошибок"
const ruPlural = new Intl.PluralRules("ru-RU");

export function pluralRu(n: number, one: string, few: string, many: string): string {
  const category = ruPlural.select(n);
  if (category === "one") return one;
  if (category === "few") return few;
  return many;
}
