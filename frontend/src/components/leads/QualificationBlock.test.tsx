import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { QualificationBlock } from "./QualificationBlock";

describe("QualificationBlock", () => {
  it("renders the score and every qualification field", () => {
    render(
      <QualificationBlock
        score={87}
        identifiedNeed="CRM-интеграция"
        estimatedBudget="500 000 ₽"
        deadline="Q3 2026"
        recommendedAction="Назначить демо на этой неделе"
      />,
    );
    expect(screen.getByText("87")).toBeInTheDocument();
    expect(screen.getByText("CRM-интеграция")).toBeInTheDocument();
    expect(screen.getByText("500 000 ₽")).toBeInTheDocument();
    expect(screen.getByText("Q3 2026")).toBeInTheDocument();
    expect(screen.getByText("Назначить демо на этой неделе")).toBeInTheDocument();
  });

  it("renders the section title and labels", () => {
    render(
      <QualificationBlock
        score={0}
        identifiedNeed=""
        estimatedBudget=""
        deadline=""
        recommendedAction=""
      />,
    );
    expect(screen.getByText("ИИ-квалификация лида")).toBeInTheDocument();
    expect(screen.getByText("Выявленная потребность")).toBeInTheDocument();
    expect(screen.getByText("Оценка бюджета")).toBeInTheDocument();
    expect(screen.getByText("Сроки")).toBeInTheDocument();
  });
});
