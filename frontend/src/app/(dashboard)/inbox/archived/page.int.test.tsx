import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import { NotificationProvider } from "@/components/notifications/NotificationProvider";
import type { Lead } from "@/lib/api";

// ArchivedLeadCard links via next/link + the page reads no router; pin
// usePathname so the page mounts without a Next router context.
vi.mock("next/navigation", () => ({
  usePathname: () => "/inbox/archived",
}));

import ArchivedLeadsPage from "./page";

function archivedLead(over: Partial<Lead> = {}): Lead {
  return {
    id: over.id ?? "l-1",
    user_id: over.user_id ?? "u-1",
    channel: over.channel ?? "telegram",
    contact_name: over.contact_name ?? "Иван Петров",
    company: over.company ?? "Acme",
    first_message: over.first_message ?? "Здравствуйте",
    status: over.status ?? "qualified",
    source_name: over.source_name ?? "Источник A",
    created_at: over.created_at ?? "2026-06-25T10:00:00Z",
    updated_at: over.updated_at ?? "2026-06-25T10:00:00Z",
    archived_at: over.archived_at ?? "2026-06-25T11:00:00Z",
  };
}

function mountWith(leads: Lead[], extra: Parameters<typeof server.use> = []) {
  server.use(
    http.get(url("/api/leads/archived"), () => HttpResponse.json(leads)),
    ...extra,
  );
}

describe("archived leads page (integration)", () => {
  it("loads archived leads from the API and renders the list", async () => {
    mountWith([
      archivedLead({ id: "a", company: "Acme" }),
      archivedLead({ id: "b", company: "Globex" }),
    ]);

    render(
      <NotificationProvider>
        <ArchivedLeadsPage />
      </NotificationProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Acme")).toBeInTheDocument();
    });
    expect(screen.getByText("Globex")).toBeInTheDocument();
  });

  it("unarchives via POST and drops the row from the list", async () => {
    let unarchiveCalled = "";
    mountWith(
      [archivedLead({ id: "keep", company: "Acme" })],
      [
        http.post(url("/api/leads/keep/unarchive"), () => {
          unarchiveCalled = "keep";
          return HttpResponse.json({ status: "active" });
        }),
      ],
    );

    render(
      <NotificationProvider>
        <ArchivedLeadsPage />
      </NotificationProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Acme")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /Разархивировать/ }));

    await waitFor(() => {
      expect(unarchiveCalled).toBe("keep");
    });
    await waitFor(() => {
      expect(screen.queryByText("Acme")).not.toBeInTheDocument();
    });
    expect(screen.getByText("Лид возвращён")).toBeInTheDocument();
  });

  it("renders the empty state when the API returns no archived leads", async () => {
    mountWith([]);

    render(
      <NotificationProvider>
        <ArchivedLeadsPage />
      </NotificationProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Архив пуст")).toBeInTheDocument();
    });
  });
});
