import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { QualificationCard } from "./QualificationCard";
import type { Qualification } from "@/lib/api";

function qual(over: Partial<Qualification> = {}): Qualification {
  return {
    id: "q-1",
    lead_id: "l-1",
    identified_need: "Интеграция с CRM",
    estimated_budget: "500к₽",
    deadline: "Q3 2026",
    score: 87,
    score_reason: "Чёткая потребность",
    recommended_action: "Назначить демо",
    provider_used: "openai",
    generated_at: "2026-06-24T10:00:00Z",
    ...over,
  };
}

describe("QualificationCard", () => {
  it("shows a loading spinner state when loading", () => {
    render(<QualificationCard qualification={null} loading={true} />);
    expect(screen.getByText(/Загрузка квалификации/)).toBeInTheDocument();
  });

  it("shows the awaiting message when there is no qualification", () => {
    render(<QualificationCard qualification={null} loading={false} />);
    expect(screen.getByText(/Ожидает квалификации ИИ/)).toBeInTheDocument();
  });

  it("renders score, need, budget, deadline and recommended action", () => {
    render(<QualificationCard qualification={qual()} loading={false} />);
    expect(screen.getByText("87")).toBeInTheDocument();
    expect(screen.getByText("Интеграция с CRM")).toBeInTheDocument();
    expect(screen.getByText("500к₽")).toBeInTheDocument();
    expect(screen.getByText("Q3 2026")).toBeInTheDocument();
    expect(screen.getByText("Назначить демо")).toBeInTheDocument();
    expect(screen.getByText("Оценка")).toBeInTheDocument();
  });

  it("does not render the awaiting or loading text when data is present", () => {
    render(<QualificationCard qualification={qual()} loading={false} />);
    expect(screen.queryByText(/Ожидает квалификации/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Загрузка квалификации/)).not.toBeInTheDocument();
  });
});
