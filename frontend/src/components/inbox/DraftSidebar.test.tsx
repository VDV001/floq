import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { DraftSidebar } from "./DraftSidebar";
import { api, type Draft } from "@/lib/api";

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    api: {
      regenerateDraft: vi.fn(),
      sendMessage: vi.fn(),
      getMessages: vi.fn(),
    },
  };
});

const draft = (over: Partial<Draft> = {}): Draft => ({
  id: "d-1",
  lead_id: "lead-1",
  body: "ИИ-сгенерированный ответ",
  created_at: "2026-06-25T10:00:00Z",
  ...over,
});

describe("DraftSidebar", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it("lets the operator write a reply from scratch when there is no AI draft", async () => {
    render(
      <DraftSidebar
        leadId="lead-1"
        draft={null}
        draftLoading={false}
        onDraftChanged={vi.fn()}
        onMessagesSent={vi.fn()}
      />
    );

    // An editable textarea must be available even without an AI draft.
    const textarea = screen.getByRole("textbox");
    expect(textarea).toBeInTheDocument();
    expect(textarea).not.toBeDisabled();
  });

  it("sends a hand-written reply verbatim without calling the AI", async () => {
    vi.mocked(api.sendMessage).mockResolvedValue({} as never);
    vi.mocked(api.getMessages).mockResolvedValue([]);
    const onMessagesSent = vi.fn();

    render(
      <DraftSidebar
        leadId="lead-1"
        draft={null}
        draftLoading={false}
        onDraftChanged={vi.fn()}
        onMessagesSent={onMessagesSent}
      />
    );

    const textarea = screen.getByRole("textbox");
    await userEvent.type(textarea, "Здравствуйте, отвечаю вручную.");
    await userEvent.click(screen.getByText("Отправить ответ"));

    await waitFor(() => {
      expect(api.sendMessage).toHaveBeenCalledWith(
        "lead-1",
        "Здравствуйте, отвечаю вручную."
      );
    });
    expect(api.regenerateDraft).not.toHaveBeenCalled();
    expect(onMessagesSent).toHaveBeenCalled();
  });

  it("does not send when the reply is empty", async () => {
    render(
      <DraftSidebar
        leadId="lead-1"
        draft={null}
        draftLoading={false}
        onDraftChanged={vi.fn()}
        onMessagesSent={vi.fn()}
      />
    );

    await userEvent.click(screen.getByText("Отправить ответ"));

    expect(api.sendMessage).not.toHaveBeenCalled();
  });

  it("clears the typed reply when switching to another lead", async () => {
    const { rerender } = render(
      <DraftSidebar
        leadId="lead-A"
        draft={null}
        draftLoading={false}
        onDraftChanged={vi.fn()}
        onMessagesSent={vi.fn()}
      />
    );

    await userEvent.type(screen.getByRole("textbox"), "секрет лида A");
    expect(screen.getByRole("textbox")).toHaveValue("секрет лида A");

    // Navigate to a different draftless lead — its reply box must be empty.
    rerender(
      <DraftSidebar
        leadId="lead-B"
        draft={null}
        draftLoading={false}
        onDraftChanged={vi.fn()}
        onMessagesSent={vi.fn()}
      />
    );

    expect(screen.getByRole("textbox")).toHaveValue("");
  });

  it("ignores a stale draft that belongs to the previous lead", async () => {
    // Parent re-fetches the draft async, so right after navigation the draft
    // prop still holds lead A's draft while leadId is already lead B.
    const staleDraft = draft({ lead_id: "lead-A", body: "черновик лида A" });
    const { rerender } = render(
      <DraftSidebar
        leadId="lead-A"
        draft={staleDraft}
        draftLoading={false}
        onDraftChanged={vi.fn()}
        onMessagesSent={vi.fn()}
      />
    );

    rerender(
      <DraftSidebar
        leadId="lead-B"
        draft={staleDraft}
        draftLoading={false}
        onDraftChanged={vi.fn()}
        onMessagesSent={vi.fn()}
      />
    );

    expect(screen.getByRole("textbox")).toHaveValue("");
  });

  it("still shows and edits an AI draft when one exists", async () => {
    vi.mocked(api.sendMessage).mockResolvedValue({} as never);
    vi.mocked(api.getMessages).mockResolvedValue([]);

    render(
      <DraftSidebar
        leadId="lead-1"
        draft={draft()}
        draftLoading={false}
        onDraftChanged={vi.fn()}
        onMessagesSent={vi.fn()}
      />
    );

    expect(screen.getByDisplayValue("ИИ-сгенерированный ответ")).toBeInTheDocument();
  });
});
