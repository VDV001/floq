import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const scrapeWebsite = vi.fn();
vi.mock("@/lib/api", () => ({
  api: {
    scrapeWebsite: (...args: unknown[]) => scrapeWebsite(...args),
  },
}));

import { WebsiteScraper } from "./WebsiteScraper";

beforeEach(() => {
  scrapeWebsite.mockReset();
});

describe("WebsiteScraper", () => {
  it("disables the search button until a URL is entered", () => {
    render(<WebsiteScraper />);
    const btn = screen.getByRole("button", { name: "Найти email" });
    expect(btn).toBeDisabled();
    fireEvent.change(screen.getByPlaceholderText("https://company.ru"), {
      target: { value: "https://acme.ru" },
    });
    expect(btn).not.toBeDisabled();
  });

  it("lists found emails with their count", async () => {
    scrapeWebsite.mockResolvedValue({ url: "https://acme.ru", emails: ["a@acme.ru", "b@acme.ru"] });
    render(<WebsiteScraper />);
    fireEvent.change(screen.getByPlaceholderText("https://company.ru"), {
      target: { value: "https://acme.ru" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Найти email" }));
    await waitFor(() => expect(screen.getByText("Найдено: 2")).toBeInTheDocument());
    expect(screen.getByText("a@acme.ru")).toBeInTheDocument();
    expect(screen.getByText("b@acme.ru")).toBeInTheDocument();
    expect(scrapeWebsite).toHaveBeenCalledWith("https://acme.ru");
  });

  it("shows a not-found message when no emails are returned", async () => {
    scrapeWebsite.mockResolvedValue({ url: "https://acme.ru", emails: [] });
    render(<WebsiteScraper />);
    fireEvent.change(screen.getByPlaceholderText("https://company.ru"), {
      target: { value: "https://acme.ru" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Найти email" }));
    await waitFor(() => expect(screen.getByText("Email не найдены на этом сайте")).toBeInTheDocument());
  });

  it("shows an error message when the scrape fails", async () => {
    scrapeWebsite.mockRejectedValue(new Error("boom"));
    render(<WebsiteScraper />);
    fireEvent.change(screen.getByPlaceholderText("https://company.ru"), {
      target: { value: "https://acme.ru" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Найти email" }));
    await waitFor(() => expect(screen.getByText("Не удалось загрузить сайт")).toBeInTheDocument());
  });
});
