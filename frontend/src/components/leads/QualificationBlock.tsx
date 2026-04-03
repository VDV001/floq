"use client";

import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Sparkles } from "lucide-react";

interface QualificationBlockProps {
  score: number;
  identifiedNeed: string;
  estimatedBudget: string;
  deadline: string;
  recommendedAction: string;
}

export function QualificationBlock({
  score,
  identifiedNeed,
  estimatedBudget,
  deadline,
  recommendedAction,
}: QualificationBlockProps) {
  return (
    <Card className="bg-white">
      <CardHeader className="flex-row items-center justify-between">
        <div className="flex items-center gap-2">
          <Sparkles className="size-5 text-[#3b6ef6]" />
          <span className="font-semibold text-[#0d1c2e]">
            ИИ-квалификация лида
          </span>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs font-medium tracking-wide text-[#6b7280] uppercase">
            Оценка
          </span>
          <div className="flex size-12 items-center justify-center rounded-full border-[3px] border-[#3b6ef6]">
            <span className="text-lg font-bold text-[#3b6ef6]">{score}</span>
          </div>
        </div>
      </CardHeader>

      <CardContent>
        <div className="grid grid-cols-3 gap-6">
          <div className="space-y-1.5">
            <span className="text-xs font-semibold tracking-wide text-[#166534] uppercase">
              Выявленная потребность
            </span>
            <p className="text-sm text-[#0d1c2e]">{identifiedNeed}</p>
          </div>
          <div className="space-y-1.5">
            <span className="text-xs font-semibold tracking-wide text-[#166534] uppercase">
              Оценка бюджета
            </span>
            <p className="text-sm text-[#0d1c2e]">{estimatedBudget}</p>
          </div>
          <div className="space-y-1.5">
            <span className="text-xs font-semibold tracking-wide text-[#166534] uppercase">
              Сроки
            </span>
            <p className="text-sm text-[#0d1c2e]">{deadline}</p>
          </div>
        </div>

        <div className="mt-4 rounded-lg bg-[#dcfce7] px-4 py-2.5">
          <p className="text-sm font-medium text-[#166534]">
            {recommendedAction}
          </p>
        </div>
      </CardContent>
    </Card>
  );
}
