import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
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

  it("renders the source filter and source analytics from the stats endpoint", async () => {
    server.use(
      http.get(url("/api/prospects"), () =>
        HttpResponse.json([
          prospect({ id: "a", name: "Иван Петров", source_name: "2GIS" }),
          prospect({ id: "b", name: "Мария Сидорова", source_name: "Яндекс" }),
        ]),
      ),
      http.get(url("/api/sources"), () => HttpResponse.json([])),
      http.get(url("/api/sources/stats"), () =>
        HttpResponse.json([
          { source_id: "s1", source_name: "2GIS", category_name: "Каталоги", prospect_count: 8, lead_count: 2, converted_count: 5 },
        ]),
      ),
    );

    render(<ProspectsPage />);
    await screen.findByText("Иван Петров");

    // SourceAnalytics renders the conversion read-model.
    expect(await screen.findByText("Конверсия по источникам")).toBeInTheDocument();
    expect(screen.getByText("50%")).toBeInTheDocument();

    // Filtering by source narrows the rows.
    fireEvent.change(screen.getByDisplayValue("Все источники"), { target: { value: "2GIS" } });
    expect(screen.queryByText("Мария Сидорова")).not.toBeInTheDocument();
    expect(screen.getByText("Иван Петров")).toBeInTheDocument();
  });

  it("withdraws consent through the API", async () => {
    const user = userEvent.setup({ delay: null });
    let patched: { status?: string } = {};
    mountWith(
      [prospect({ id: "a", name: "Иван Петров", consent_status: "obtained" })],
      [
        http.post(url("/api/prospects/a/consent"), async ({ request }) => {
          patched = { status: ((await request.json()) as { status: string }).status };
          return HttpResponse.json({ consent_status: "withdrawn" });
        }),
      ],
    );

    render(<ProspectsPage />);
    await screen.findByText("Иван Петров");
    await user.click(screen.getByTitle("Отозвать согласие"));

    expect(await screen.findByText("Согласие отозвано")).toBeInTheDocument();
    expect(patched).toEqual({ status: "withdrawn" });
  });

  it("imports a CSV file and reports the count, then refetches", async () => {
    const user = userEvent.setup({ delay: null });
    const alertSpy = vi.spyOn(window, "alert").mockImplementation(() => {});
    let fetchCount = 0;
    server.use(
      http.get(url("/api/prospects"), () => {
        fetchCount += 1;
        return HttpResponse.json(fetchCount > 1 ? [prospect({ id: "a", name: "Импортированный" })] : []);
      }),
      http.get(url("/api/sources"), () => HttpResponse.json([])),
      http.get(url("/api/sources/stats"), () => HttpResponse.json([])),
      http.post(url("/api/prospects/import"), () => HttpResponse.json({ imported: 7 })),
    );

    render(<ProspectsPage />);
    await waitFor(() => expect(fetchCount).toBe(1));

    const file = new File(["name,email\nA,a@x.io"], "prospects.csv", { type: "text/csv" });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(input, file);

    await waitFor(() => expect(alertSpy).toHaveBeenCalledWith("Импортировано 7 проспектов"));
    expect(await screen.findByText("Импортированный")).toBeInTheDocument();
    alertSpy.mockRestore();
  });

  it("verifies the batch and shows the resulting toast", async () => {
    const user = userEvent.setup({ delay: null });
    mountWith(
      [prospect({ id: "a", name: "Иван Петров" })],
      [http.post(url("/api/verify/batch"), () => HttpResponse.json({ verified: 4 }))],
    );

    render(<ProspectsPage />);
    await screen.findByText("Иван Петров");
    await user.click(screen.getByRole("button", { name: /Проверить базу/ }));

    // The hook gates the toast behind a 2.5s minimum-visible spinner.
    expect(await screen.findByText("Проверено 4 проспектов", undefined, { timeout: 4000 })).toBeInTheDocument();
  });
});
