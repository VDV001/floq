import { describe, it, expect } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import { prospect } from "@/__tests__/integration/fixtures";
import { NotificationProvider } from "@/components/notifications/NotificationProvider";
import type { Prospect, Sequence, SequenceStep } from "@/lib/api";

import SequencesPage from "./page";

// Integration: real SequencesPage + useSequences/useSequenceSteps/useProspects
// + lib/api.ts, network via MSW.
//
// On mount the page fires three GETs:
//   /api/sequences            (useSequences -> list, auto-selects first)
//   /api/prospects            (useProspects -> ProspectSelector list)
//   /api/sequences/:id        (useSequenceSteps -> steps of the selected seq)
function sequence(over: Partial<Sequence> = {}): Sequence {
  return {
    id: over.id ?? "s-1",
    user_id: over.user_id ?? "u-1",
    name: over.name ?? "Холодная цепочка",
    is_active: over.is_active ?? true,
    require_approval: over.require_approval ?? false,
    created_at: over.created_at ?? "2026-06-01T00:00:00Z",
  };
}

function step(over: Partial<SequenceStep> = {}): SequenceStep {
  return {
    id: over.id ?? "st-1",
    sequence_id: over.sequence_id ?? "s-1",
    step_order: over.step_order ?? 1,
    delay_days: over.delay_days ?? 0,
    prompt_hint: over.prompt_hint ?? "Первое касание, представиться",
    body: over.body ?? "",
    channel: over.channel ?? "email",
    created_at: over.created_at ?? "2026-06-01T00:00:00Z",
  };
}

function renderPage() {
  return render(
    <NotificationProvider>
      <SequencesPage />
    </NotificationProvider>,
  );
}

function mountWith(opts: {
  sequences: Sequence[];
  prospects?: Prospect[];
  stepsBySeq?: Record<string, SequenceStep[]>;
  extra?: Parameters<typeof server.use>;
}) {
  const stepsBySeq = opts.stepsBySeq ?? {};
  server.use(
    http.get(url("/api/sequences"), () => HttpResponse.json(opts.sequences)),
    http.get(url("/api/prospects"), () => HttpResponse.json(opts.prospects ?? [])),
    http.get(url("/api/sequences/:id"), ({ params }) => {
      const id = params.id as string;
      const seq = opts.sequences.find((s) => s.id === id) ?? sequence({ id });
      return HttpResponse.json({ sequence: seq, steps: stepsBySeq[id] ?? [] });
    }),
    ...(opts.extra ?? []),
  );
}

describe("sequences page (integration)", () => {
  it("loads sequences from the API and renders the list plus the selected sequence's steps", async () => {
    mountWith({
      sequences: [
        sequence({ id: "s-1", name: "Холодная цепочка", is_active: true }),
        sequence({ id: "s-2", name: "Реактивация", is_active: false }),
      ],
      prospects: [prospect({ id: "p-1", name: "Иван Петров" })],
      stepsBySeq: {
        "s-1": [step({ id: "st-1", step_order: 1, prompt_hint: "Первое касание" })],
      },
    });

    renderPage();

    // Both sequence cards render from the list response.
    expect(await screen.findByText("Холодная цепочка")).toBeInTheDocument();
    expect(screen.getByText("Реактивация")).toBeInTheDocument();

    // First sequence is auto-selected -> its name shows in the steps header
    // and its step (fetched via /api/sequences/s-1) is rendered.
    expect(await screen.findByText("Первое касание")).toBeInTheDocument();
    expect(screen.getByText("— Холодная цепочка")).toBeInTheDocument();

    // Prospect list rendered from /api/prospects.
    expect(screen.getByText("Иван Петров")).toBeInTheDocument();
    expect(screen.getByText("Проспекты (1)")).toBeInTheDocument();
  });

  it("loads a different sequence's steps when another sequence card is selected", async () => {
    const user = userEvent.setup({ delay: null });
    mountWith({
      sequences: [
        sequence({ id: "s-1", name: "Холодная цепочка" }),
        sequence({ id: "s-2", name: "Реактивация" }),
      ],
      stepsBySeq: {
        "s-1": [step({ id: "st-1", step_order: 1, prompt_hint: "Первое касание" })],
        "s-2": [step({ id: "st-2", sequence_id: "s-2", step_order: 1, prompt_hint: "Возврат клиента" })],
      },
    });

    renderPage();

    // Auto-selected s-1's step is shown first.
    expect(await screen.findByText("Первое касание")).toBeInTheDocument();

    await user.click(screen.getByText("Реактивация"));

    // Selecting s-2 triggers a fresh /api/sequences/s-2 fetch with its steps.
    expect(await screen.findByText("Возврат клиента")).toBeInTheDocument();
    expect(screen.getByText("— Реактивация")).toBeInTheDocument();
    expect(screen.queryByText("Первое касание")).not.toBeInTheDocument();
  });

  it("creates a sequence via POST and renders it in the list", async () => {
    const user = userEvent.setup({ delay: null });
    let posted: { name?: string } = {};
    mountWith({
      sequences: [],
      extra: [
        http.post(url("/api/sequences"), async ({ request }) => {
          posted = (await request.json()) as { name: string };
          return HttpResponse.json(sequence({ id: "s-new", name: posted.name, is_active: false }));
        }),
      ],
    });

    renderPage();

    // Empty-state copy from SequenceList confirms the initial load resolved.
    expect(await screen.findByText("Нет секвенций")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Новая секвенция" }));

    fireEvent.change(screen.getByPlaceholderText("Название секвенции..."), {
      target: { value: "Весенняя рассылка" },
    });
    await user.click(screen.getByRole("button", { name: "Создать" }));

    // POST body reached the API and the created sequence renders in the list.
    expect(await screen.findByText("Весенняя рассылка")).toBeInTheDocument();
    expect(posted).toEqual({ name: "Весенняя рассылка" });
  });

  it("cancels the new-sequence form on Escape", async () => {
    const user = userEvent.setup({ delay: null });
    mountWith({ sequences: [] });
    renderPage();
    await screen.findByText("Нет секвенций");

    await user.click(screen.getByRole("button", { name: "Новая секвенция" }));
    const input = screen.getByPlaceholderText("Название секвенции...");
    fireEvent.change(input, { target: { value: "Черновик" } });
    fireEvent.keyDown(input, { key: "Escape" });

    expect(screen.queryByPlaceholderText("Название секвенции...")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Новая секвенция" })).toBeInTheDocument();
  });

  it("creates a sequence by pressing Enter in the name field", async () => {
    const user = userEvent.setup({ delay: null });
    mountWith({
      sequences: [],
      extra: [
        http.post(url("/api/sequences"), async ({ request }) => {
          const body = (await request.json()) as { name: string };
          return HttpResponse.json(sequence({ id: "s-new", name: body.name }));
        }),
      ],
    });
    renderPage();
    await screen.findByText("Нет секвенций");

    await user.click(screen.getByRole("button", { name: "Новая секвенция" }));
    const input = screen.getByPlaceholderText("Название секвенции...");
    fireEvent.change(input, { target: { value: "Enter-цепочка" } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(await screen.findByText("Enter-цепочка")).toBeInTheDocument();
  });

  it("toggles a sequence active state through the API", async () => {
    const user = userEvent.setup({ delay: null });
    let toggled: { is_active?: boolean } = {};
    mountWith({
      sequences: [sequence({ id: "s-1", name: "Холодная цепочка", is_active: true })],
      extra: [
        http.patch(url("/api/sequences/s-1/toggle"), async ({ request }) => {
          toggled = (await request.json()) as { is_active: boolean };
          return HttpResponse.json({});
        }),
      ],
    });
    renderPage();
    await screen.findByText("Холодная цепочка");

    await user.click(screen.getByRole("switch", { name: "Активность секвенции" }));
    await waitFor(() => expect(toggled).toEqual({ is_active: false }));
  });

  it("toggles the per-sequence approval gate through the API", async () => {
    const user = userEvent.setup({ delay: null });
    let body: { require_approval?: boolean } = {};
    mountWith({
      sequences: [sequence({ id: "s-1", name: "Холодная цепочка", require_approval: false })],
      extra: [
        http.patch(url("/api/sequences/s-1/approval"), async ({ request }) => {
          body = (await request.json()) as { require_approval: boolean };
          return HttpResponse.json({ require_approval: true });
        }),
      ],
    });
    renderPage();
    await screen.findByText("Холодная цепочка");

    await user.click(screen.getByRole("switch", { name: "Требовать одобрение перед отправкой" }));
    await waitFor(() => expect(body).toEqual({ require_approval: true }));
  });

  it("renames a sequence through the edit dialog", async () => {
    const user = userEvent.setup({ delay: null });
    let put: { name?: string } = {};
    mountWith({
      sequences: [sequence({ id: "s-1", name: "Старое имя" })],
      extra: [
        http.put(url("/api/sequences/s-1"), async ({ request }) => {
          put = (await request.json()) as { name: string };
          return HttpResponse.json(sequence({ id: "s-1", name: put.name! }));
        }),
      ],
    });
    renderPage();
    await screen.findByText("Старое имя");

    await user.click(screen.getByRole("button", { name: "Редактировать" }));
    const input = screen.getByDisplayValue("Старое имя");
    fireEvent.change(input, { target: { value: "Новое имя" } });
    await user.click(screen.getByRole("button", { name: "Сохранить" }));

    await waitFor(() => expect(put).toEqual({ name: "Новое имя" }));
    expect(await screen.findByText("Новое имя")).toBeInTheDocument();
  });

  it("deletes a sequence after confirming in the dialog", async () => {
    const user = userEvent.setup({ delay: null });
    let deleted = false;
    mountWith({
      sequences: [sequence({ id: "s-1", name: "На удаление" })],
      extra: [
        http.delete(url("/api/sequences/s-1"), () => {
          deleted = true;
          return new HttpResponse(null, { status: 204 });
        }),
      ],
    });
    renderPage();
    await screen.findByText("На удаление");

    await user.click(screen.getByRole("button", { name: "Удалить" }));
    // ConfirmDialog opens (heading "Удалить секвенцию"); now two "Удалить"
    // buttons exist — the dialog's confirm is the last one.
    expect(await screen.findByText("Удалить секвенцию")).toBeInTheDocument();
    const deleteButtons = screen.getAllByRole("button", { name: "Удалить" });
    await user.click(deleteButtons[deleteButtons.length - 1]);

    await waitFor(() => expect(deleted).toBe(true));
    expect(screen.queryByText("На удаление")).not.toBeInTheDocument();
  });

  it("adds a step through the add-step form and refetches the timeline", async () => {
    const user = userEvent.setup({ delay: null });
    let posted: unknown = null;
    const steps: SequenceStep[] = [];
    server.use(
      http.get(url("/api/sequences"), () => HttpResponse.json([sequence({ id: "s-1", name: "Цепочка" })])),
      http.get(url("/api/prospects"), () => HttpResponse.json([])),
      http.get(url("/api/sequences/:id"), () =>
        HttpResponse.json({ sequence: sequence({ id: "s-1" }), steps: [...steps] }),
      ),
      http.post(url("/api/sequences/s-1/steps"), async ({ request }) => {
        posted = await request.json();
        steps.push(step({ id: "st-new", prompt_hint: "новый шаг" }));
        return HttpResponse.json(steps[steps.length - 1]);
      }),
    );
    renderPage();
    await screen.findByText("Цепочка");

    await user.click(await screen.findByRole("button", { name: /Добавить шаг/ }));
    await user.click(screen.getByRole("button", { name: "Добавить" }));

    await waitFor(() => expect(posted).toMatchObject({ channel: "email", step_order: 1 }));
    expect(await screen.findByText("новый шаг")).toBeInTheDocument();
  });

  it("launches the sequence for the selected prospects", async () => {
    const user = userEvent.setup({ delay: null });
    let launched: { prospect_ids?: string[]; send_now?: boolean } = {};
    mountWith({
      sequences: [sequence({ id: "s-1", name: "Цепочка" })],
      prospects: [prospect({ id: "p-1", name: "Иван Петров", status: "new" })],
      extra: [
        http.post(url("/api/sequences/s-1/launch"), async ({ request }) => {
          launched = (await request.json()) as { prospect_ids: string[]; send_now: boolean };
          return HttpResponse.json({});
        }),
      ],
    });
    renderPage();
    await screen.findByText("Иван Петров");

    await user.click(screen.getByRole("button", { name: "Выбрать все" }));
    // First click reveals launch options, second click fires the launch.
    const launchBtn = screen.getByRole("button", { name: /Запустить \(1\)/ });
    await user.click(launchBtn);
    await user.click(screen.getByRole("button", { name: /Запустить \(1\)/ }));

    await waitFor(() => expect(launched.prospect_ids).toEqual(["p-1"]));
  });
});
