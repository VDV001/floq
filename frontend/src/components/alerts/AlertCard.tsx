"use client";

import { Mail, Clock, X } from "lucide-react";
import { Button } from "@/components/ui/button";

interface AlertCardProps {
  name: string;
  company: string;
  title: string;
  initials: string;
  lastContact: string;
  action: string;
  avatarColor: string;
}

export function AlertCard({
  name,
  company,
  title,
  initials,
  lastContact,
  action,
  avatarColor,
}: AlertCardProps) {
  return (
    <div className="flex items-center gap-4 rounded-lg border border-gray-100 bg-white px-4 py-3">
      <div
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-sm font-semibold text-white"
        style={{ backgroundColor: avatarColor }}
      >
        {initials}
      </div>

      {/* Name + Company */}
      <div className="min-w-0 w-44">
        <div className="font-semibold text-[#0d1c2e]">{name}</div>
        <div className="text-xs text-[#6b7280] truncate">{company} &middot; {title}</div>
      </div>

      {/* Last Contact */}
      <div className="w-28 shrink-0">
        <div className="text-[10px] font-semibold uppercase tracking-wide text-[#6b7280]">Последний контакт</div>
        <div className="text-sm text-[#0d1c2e]">{lastContact}</div>
      </div>

      {/* Action */}
      <div className="flex-1 min-w-0">
        <span className="text-xs text-[#6b7280]"><span className="font-medium">Действие:</span> {action}</span>
      </div>

      <div className="flex shrink-0 items-center gap-1">
        <Button variant="ghost" size="icon-sm" className="text-[#3b6ef6] hover:bg-blue-50">
          <Mail className="size-4" />
        </Button>
        <Button variant="ghost" size="icon-sm" className="text-[#6b7280] hover:bg-gray-50">
          <Clock className="size-4" />
        </Button>
        <Button variant="ghost" size="icon-sm" className="text-[#6b7280] hover:bg-gray-50">
          <X className="size-4" />
        </Button>
      </div>
    </div>
  );
}
