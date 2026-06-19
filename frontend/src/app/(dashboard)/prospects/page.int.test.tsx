import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import { prospect } from "@/__tests__/integration/fixtures";
import type { Prospect } from "@/lib/api";

import ProspectsPage from "./page";

// Integration: real ProspectsPage + useProspectsPage + lib/api.ts, network via MSW.
function mountWith(prospects: Prospect[], extra: Parameters<typeof server.use> = []) {
  server.use(
    http.get(url("/api/prospects"), () => HttpResponse.json(prospects)),
    http.get(url("/api/sources/stats"), () => HttpResponse.json([])),
    // AddProspectForm loads the source list on mount.
    http.get(url("/api/sources"), () => HttpResponse.json([])),
    ...extra,
  );
}

describe("prospects page (integration)", () => {
  it("loads prospects from the API and renders them with the count", async () => {
    mountWith([
      prospect({ id: "a", name: "Иван Петров", company: "Acme" }),
      prospect({ id: "b", name: "Мария Сидорова", company: "Globex" }),
    ]);

    render(<ProspectsPage />);

    expect(await screen.findByText("Иван Петров")).toBeInTheDocument();
    expect(screen.getByText("Мария Сидорова")).toBeInTheDocument();
    expect(screen.getByText("2 контактов")).toBeInTheDocument();
  });

  it("filters the rendered rows by the search query", async () => {
    mountWith([
      prospect({ id: "a", name: "Иван Петров" }),
      prospect({ id: "b", name: "Мария Сидорова" }),
    ]);

    render(<ProspectsPage />);
    await screen.findByText("Иван Петров");

    fireEvent.change(screen.getByPlaceholderText("Поиск проспектов..."), {
      target: { value: "Мария" },
    });

    expect(screen.queryByText("Иван Петров")).not.toBeInTheDocument();
    expect(screen.getByText("Мария Сидорова")).toBeInTheDocument();
    expect(screen.getByText("2 контактов, найдено 1")).toBeInTheDocument();
  });

  it("toggles consent through the API and shows a success toast", async () => {
    const user = userEvent.setup({ delay: null });
    let patched: { id?: string; status?: string } = {};
    mountWith(
      [prospect({ id: "a", name: "Иван Петров", consent_status: "none" })],
      [
        http.post(url("/api/prospects/a/consent"), async ({ request }) => {
          patched = { id: "a", status: ((await request.json()) as { status: string }).status };
          return HttpResponse.json({ consent_status: "obtained" });
        }),
      ],
    );

    render(<ProspectsPage />);
    await screen.findByText("Иван Петров");

    await user.click(screen.getByTitle("Отметить согласие"));

    expect(await screen.findByText("Согласие отмечено")).toBeInTheDocument();
    expect(patched).toEqual({ id: "a", status: "obtained" });
  });

  it("paginates when there are more than one page of prospects", async () => {
    const user = userEvent.setup({ delay: null });
    // 16 prospects -> PER_PAGE 15 -> 2 pages.
    const many = Array.from({ length: 16 }, (_, i) =>
      prospect({ id: `p${i}`, name: `Контакт ${String(i).padStart(2, "0")}`, email: `c${i}@x.io` }),
    );
    mountWith(many);

    render(<ProspectsPage />);
    await screen.findByText("Контакт 00");

    // 15 body rows on page 1 (+1 header row).
    const firstPageRows = screen.getAllByRole("row").length;
    expect(firstPageRows).toBe(16);
    expect(screen.getByText("1–15 из 16 проспектов")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "2" }));
    expect(screen.getByText("16–16 из 16 проспектов")).toBeInTheDocument();
  });
});
