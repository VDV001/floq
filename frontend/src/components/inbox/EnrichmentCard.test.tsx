import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { EnrichmentCard } from "./EnrichmentCard";
import type { Enrichment } from "@/lib/api";

function enr(over: Partial<Enrichment> = {}): Enrichment {
  return {
    domain: "acme.ru",
    status: "enriched",
    profile: {
      title: "Acme LLC",
      description: "Мы делаем виджеты",
      emails: ["info@acme.ru"],
      phones: ["+74951234567"],
      socials: ["https://t.me/acme"],
    },
    enrichedAt: "2026-06-26T10:00:00Z",
    ...over,
  };
}

describe("EnrichmentCard", () => {
  it("shows a loading state", () => {
    render(<EnrichmentCard enrichment={null} loading={true} />);
    expect(screen.getByText(/Загрузка данных о компании/)).toBeInTheDocument();
  });

  it("renders title, description, contacts and socials when enriched", () => {
    render(<EnrichmentCard enrichment={enr()} loading={false} />);
    expect(screen.getByText("Acme LLC")).toBeInTheDocument();
    expect(screen.getByText("Мы делаем виджеты")).toBeInTheDocument();
    expect(screen.getByText("info@acme.ru")).toBeInTheDocument();
    expect(screen.getByText("+74951234567")).toBeInTheDocument();
    expect(screen.getByText(/t\.me\/acme/)).toBeInTheDocument();
  });

  it("renders industry and a human-readable company size", () => {
    render(<EnrichmentCard enrichment={enr({ profile: { title: "Acme LLC", description: "", emails: [], phones: [], socials: [], industry: "финтех", companySize: "medium" } })} loading={false} />);
    expect(screen.getByText(/финтех/i)).toBeInTheDocument();
    expect(screen.getByText(/11–50/)).toBeInTheDocument();
  });

  it("treats a profile with only industry as having data", () => {
    render(<EnrichmentCard enrichment={enr({ profile: { title: "", description: "", emails: [], phones: [], socials: [], industry: "логистика" } })} loading={false} />);
    expect(screen.getByText(/логистика/)).toBeInTheDocument();
    expect(screen.queryByText(/Нет данных о компании/)).not.toBeInTheDocument();
  });

  it("renders legal/registry details when present", () => {
    render(<EnrichmentCard enrichment={enr({ profile: { title: "Acme LLC", description: "", emails: [], phones: [], socials: [], legal: { inn: "7707083893", ogrn: "1027700132195", address: "г Москва", okved: "62.01", status: "ACTIVE" } } })} loading={false} />);
    expect(screen.getByText("7707083893")).toBeInTheDocument();
    expect(screen.getByText("1027700132195")).toBeInTheDocument();
    expect(screen.getByText(/г Москва/)).toBeInTheDocument();
  });

  it("treats a profile with only legal details as having data", () => {
    render(<EnrichmentCard enrichment={enr({ profile: { title: "", description: "", emails: [], phones: [], socials: [], legal: { inn: "7707083893" } } })} loading={false} />);
    expect(screen.getByText("7707083893")).toBeInTheDocument();
    expect(screen.queryByText(/Нет данных о компании/)).not.toBeInTheDocument();
  });

  it("shows a pending state while the worker is scraping", () => {
    render(<EnrichmentCard enrichment={enr({ status: "pending", profile: { title: "", description: "", emails: [], phones: [], socials: [] } })} loading={false} />);
    expect(screen.getByText(/Собираем данные о компании/)).toBeInTheDocument();
  });

  it("shows a no-data state when status is none", () => {
    render(<EnrichmentCard enrichment={enr({ status: "none", profile: { title: "", description: "", emails: [], phones: [], socials: [] } })} loading={false} />);
    expect(screen.getByText(/Нет данных о компании/)).toBeInTheDocument();
  });

  it("treats an enriched-but-empty profile as no data", () => {
    render(<EnrichmentCard enrichment={enr({ status: "enriched", profile: { title: "", description: "", emails: [], phones: [], socials: [] } })} loading={false} />);
    expect(screen.getByText(/Нет данных о компании/)).toBeInTheDocument();
  });
});
