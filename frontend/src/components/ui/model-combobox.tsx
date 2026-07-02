"use client";

import { useState, useEffect, useRef } from "react";
import { ChevronDown } from "lucide-react";
import type { AIModelOption } from "@/lib/api";

interface ModelComboboxProps {
  value: string;
  onChange: (model: string) => void;
  options: AIModelOption[];
  loading?: boolean;
  placeholder?: string;
}

// RED stub — real combobox lands in the GREEN commit.
export function ModelCombobox({ value, placeholder }: ModelComboboxProps) {
  return (
    <button type="button">
      {value || placeholder || "Модель"}
    </button>
  );
}
