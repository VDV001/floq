import type { Prospect } from "./api";

// launchBlockReason mirrors the email half of the backend's CanLaunchSequence
// rule (#221): a prospect whose email is invalid, or set-but-unverified, is
// silently skipped at launch. Returning the reason lets the UI mark it (red)
// so the user understands why nothing was queued instead of seeing a false
// success. Terminal statuses (converted/opted_out/…) already carry their own
// status badge, so they are intentionally not repeated here.
export function launchBlockReason(p: Prospect): string | null {
  if (p.verify_status === "invalid") return "email невалиден";
  if (p.verify_status === "not_checked" && p.email !== "") return "email не проверен";
  return null;
}
