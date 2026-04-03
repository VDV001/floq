"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { Send, Pencil, RefreshCw } from "lucide-react";

interface AIDraftPanelProps {
  draftBody: string;
  onEdit: () => void;
  onSend: () => void;
}

export function AIDraftPanel({ draftBody, onEdit, onSend }: AIDraftPanelProps) {
  return (
    <div className="space-y-6">
      {/* AI Reply Draft card */}
      <Card className="bg-white">
        <CardHeader className="flex-row items-center justify-between">
          <CardTitle className="text-sm font-semibold text-[#0d1c2e]">
            ИИ-черновик ответа
          </CardTitle>
          <div className="flex-1" />
          <Badge className="bg-[#3b6ef6]/10 text-[#3b6ef6] border-transparent text-[10px] font-bold tracking-wider uppercase">
            Умный черновик
          </Badge>
          <button className="flex size-6 items-center justify-center rounded text-[#6b7280] hover:bg-gray-100">
            <RefreshCw className="size-3.5" />
          </button>
        </CardHeader>

        <CardContent className="space-y-4">
          <div className="space-y-3 text-sm leading-relaxed text-[#0d1c2e]">
            {draftBody.split("\n\n").map((paragraph, i) => (
              <p key={i}>{paragraph}</p>
            ))}
          </div>

          <div className="flex flex-col gap-2">
            <Button variant="outline" className="w-full gap-1.5" onClick={onEdit}>
              <Pencil className="size-3.5" />
              Редактировать
            </Button>
            <Button
              className="w-full gap-1.5 bg-[#3b6ef6] text-white hover:bg-[#3b6ef6]/90"
              onClick={onSend}
            >
              <Send className="size-3.5" />
              Отправить
            </Button>
          </div>
        </CardContent>
      </Card>

      <Separator />

      {/* Automation Settings */}
      <div className="space-y-4 px-1">
        <h3 className="text-xs font-semibold tracking-wider text-[#6b7280] uppercase">
          Настройки автоматизации
        </h3>

        <div className="flex items-center justify-between">
          <span className="text-sm text-[#0d1c2e]">
            Авто-фоллоуапы
          </span>
          <Switch defaultChecked />
        </div>

        <div className="flex items-center justify-between">
          <span className="text-sm text-[#0d1c2e]">
            Согласование черновиков
          </span>
          <Switch defaultChecked />
        </div>
      </div>
    </div>
  );
}
