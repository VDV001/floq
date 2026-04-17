export { getTimeAgo, getInitials } from "@/lib/format";

export function getSilentDays(dateStr: string): number {
  const diff = Date.now() - new Date(dateStr).getTime();
  return Math.max(1, Math.floor(diff / (1000 * 60 * 60 * 24)));
}
