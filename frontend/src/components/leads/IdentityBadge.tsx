"use client";

import { useState, useCallback } from "react";
import Link from "next/link";
import { Mail, Phone, Send as SendIcon, Copy, Check, Users } from "lucide-react";
import { cn } from "@/lib/utils";
import type { IdentitySummary } from "@/lib/api";

interface IdentityBadgeProps {
  identity: IdentitySummary | undefined;
  currentLeadId: string;
  className?: string;
}

interface IdentifierPillProps {
  icon: typeof Mail;
  label: string;
  value: string;
  ariaLabel: string;
}

function IdentifierPill({ icon: Icon, label, value, ariaLabel }: IdentifierPillProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      const timer = setTimeout(() => setCopied(false), 1500);
      return () => clearTimeout(timer);
    } catch {
      // Clipboard write can fail in insecure contexts (http://, iframes).
      // Silently degrade — the value is still visible to the user.
    }
  }, [value]);

  return (
    <button
      type="button"
      onClick={handleCopy}
      aria-label={ariaLabel}
      className="group inline-flex items-center gap-1.5 rounded-full border border-[#dbe1ff] bg-[#eff4ff] px-3 py-1 text-xs font-medium text-[#0d1c2e] transition-colors hover:bg-[#dbe1ff] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#3b6ef6]"
    >
      <Icon className="size-3.5 text-[#737686]" aria-hidden="true" />
      <span className="sr-only">{label}: </span>
      <span>{value}</span>
      <span className="text-[#737686] transition-colors group-hover:text-[#0d1c2e]" aria-hidden="true">
        {copied ? <Check className="size-3.5 text-[#22c55e]" /> : <Copy className="size-3 opacity-0 group-hover:opacity-100" />}
      </span>
      {copied && (
        <span className="sr-only" role="status" aria-live="polite">
          {label} copied to clipboard
        </span>
      )}
    </button>
  );
}

function SiblingLink({ otherLeadCount }: { otherLeadCount: number }) {
  if (otherLeadCount < 1) return null;
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full bg-[#fff7ed] px-3 py-1 text-xs font-medium text-[#b45309] ring-1 ring-inset ring-[#fed7aa]"
      aria-label={`This contact is also reachable through ${otherLeadCount} other lead${otherLeadCount === 1 ? "" : "s"}`}
    >
      <Users className="size-3.5" aria-hidden="true" />
      <span>{otherLeadCount === 1 ? "+1 связанный лид" : `+${otherLeadCount} связанных лида`}</span>
    </span>
  );
}

/**
 * IdentityBadge renders the unified-identity context of a lead: each
 * canonical identifier (email/phone/telegram) becomes a click-to-copy
 * pill, and a sibling counter surfaces when other leads share the
 * same identity (cross-channel dedup signal).
 *
 * Returns `null` when the lead has no Identity yet — UI stays clean
 * for single-channel leads or pre-backfill rows.
 */
export function IdentityBadge({ identity, currentLeadId, className }: IdentityBadgeProps) {
  if (!identity) return null;

  const hasIdentifier = Boolean(identity.email || identity.phone || identity.telegram_username);
  if (!hasIdentifier) return null;

  const otherLeads = identity.linked_lead_ids.filter((id) => id !== currentLeadId).length;

  return (
    <section
      aria-label="Связанные каналы контакта"
      className={cn("flex flex-wrap items-center gap-2", className)}
      data-testid="identity-badge"
    >
      {identity.email && (
        <IdentifierPill icon={Mail} label="Email" value={identity.email} ariaLabel={`Email ${identity.email} — нажмите чтобы скопировать`} />
      )}
      {identity.phone && (
        <IdentifierPill icon={Phone} label="Phone" value={identity.phone} ariaLabel={`Телефон ${identity.phone} — нажмите чтобы скопировать`} />
      )}
      {identity.telegram_username && (
        <IdentifierPill
          icon={SendIcon}
          label="Telegram"
          value={`@${identity.telegram_username}`}
          ariaLabel={`Telegram @${identity.telegram_username} — нажмите чтобы скопировать`}
        />
      )}
      <SiblingLink otherLeadCount={otherLeads} />
      <Link
        href="#aggregated-timeline"
        className="sr-only focus:not-sr-only focus:rounded-md focus:bg-[#0d1c2e] focus:px-2 focus:py-1 focus:text-xs focus:text-white"
      >
        Перейти к объединённой переписке
      </Link>
    </section>
  );
}
