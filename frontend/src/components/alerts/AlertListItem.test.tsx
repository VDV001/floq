import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { AlertListItem } from "./AlertListItem";
import type { Lead } from "@/lib/api";

function lead(over: Partial<Lead> = {}): Lead {
  return {
    id: "l-1",
    user_id: "u-1",
    channel: "telegram",
    contact_name: "Иван Петров",
    company: "Acme",
    first_message: "Здравствуйте, интересует продукт",
    status: "new",
    created_at: "2026-06-20T10:00:00Z",
    updated_at: "2026-06-24T10:00:00Z",
    ...over,
  };
}

describe("AlertListItem", () => {
  it("renders contact name, initials and company with telegram channel", () => {
    render(<AlertListItem alert={lead()} />);
    expect(screen.getByText("Иван Петров")).toBeInTheDocument();
    expect(screen.getByText("ИП")).toBeInTheDocument();
    expect(screen.getByText(/Acme/)).toBeInTheDocument();
    expect(screen.getByText(/Telegram/)).toBeInTheDocument();
  });

  it("shows Email label when channel is email", () => {
    render(<AlertListItem alert={lead({ channel: "email" })} />);
    expect(screen.getByText(/Email/)).toBeInTheDocument();
  });

  it("falls back to dash when company is empty", () => {
    render(<AlertListItem alert={lead({ company: "" })} />);
    expect(screen.getByText(/—/)).toBeInTheDocument();
  });

  it("truncates a long first message preview with ellipsis", () => {
    const long = "a".repeat(120);
    render(<AlertListItem alert={lead({ first_message: long })} />);
    expect(
      screen.getByText(new RegExp(`Напомнить о: "${"a".repeat(80)}\\.\\.\\."`))
    ).toBeInTheDocument();
  });

  it("shows short first message preview without ellipsis", () => {
    render(<AlertListItem alert={lead({ first_message: "Короткое" })} />);
    expect(screen.getByText(/Напомнить о: "Короткое"/)).toBeInTheDocument();
  });

  it("shows fallback action text when first message is empty", () => {
    render(<AlertListItem alert={lead({ first_message: "" })} />);
    expect(
      screen.getByText(/Связаться с лидом для продолжения диалога/)
    ).toBeInTheDocument();
  });
});
