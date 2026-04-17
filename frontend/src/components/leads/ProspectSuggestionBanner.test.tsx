import { render, screen, waitFor, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ProspectSuggestionBanner } from "./ProspectSuggestionBanner";
import { api, type ProspectSuggestion } from "@/lib/api";

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    api: {
      getProspectSuggestions: vi.fn(),
      linkProspect: vi.fn(),
      dismissProspectSuggestion: vi.fn(),
    },
  };
});

const suggestion = (over: Partial<ProspectSuggestion> = {}): ProspectSuggestion => ({
  prospect_id: "p-1",
  name: "Даниил",
  company: "Floq",
  email: "dan@floq.dev",
  telegram_username: "dan_tg",
  source_name: "БК Магнат",
  status: "new",
  confidence: "high",
  ...over,
});

describe("ProspectSuggestionBanner", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when no suggestions returned", async () => {
    vi.mocked(api.getProspectSuggestions).mockResolvedValue([]);
    const { container } = render(<ProspectSuggestionBanner leadId="lead-1" />);
    await waitFor(() => expect(api.getProspectSuggestions).toHaveBeenCalledWith("lead-1"));
    expect(container.firstChild).toBeNull();
  });

  it("renders nothing when the API call fails", async () => {
    vi.mocked(api.getProspectSuggestions).mockRejectedValue(new Error("boom"));
    const { container } = render(<ProspectSuggestionBanner leadId="lead-1" />);
    await waitFor(() => expect(api.getProspectSuggestions).toHaveBeenCalled());
    expect(container.firstChild).toBeNull();
  });

  it("renders a suggestion card with confidence label and contact info", async () => {
    vi.mocked(api.getProspectSuggestions).mockResolvedValue([suggestion()]);
    render(<ProspectSuggestionBanner leadId="lead-1" />);

    expect(await screen.findByText("Даниил")).toBeInTheDocument();
    expect(screen.getByText(/Floq/)).toBeInTheDocument();
    expect(screen.getByText(/Высокая уверенность/)).toBeInTheDocument();
    expect(screen.getByText("dan@floq.dev")).toBeInTheDocument();
    expect(screen.getByText("@dan_tg")).toBeInTheDocument();
    expect(screen.getByText("БК Магнат")).toBeInTheDocument();
  });

  it("renders different confidence labels", async () => {
    vi.mocked(api.getProspectSuggestions).mockResolvedValue([
      suggestion({ prospect_id: "a", confidence: "high" }),
      suggestion({ prospect_id: "b", confidence: "medium" }),
      suggestion({ prospect_id: "c", confidence: "low" }),
    ]);
    render(<ProspectSuggestionBanner leadId="lead-1" />);

    expect(await screen.findByText("Высокая уверенность")).toBeInTheDocument();
    expect(screen.getByText("Средняя уверенность")).toBeInTheDocument();
    expect(screen.getByText("Низкая уверенность")).toBeInTheDocument();
  });

  it("calls linkProspect and removes the item on Связать", async () => {
    const user = userEvent.setup();
    vi.mocked(api.getProspectSuggestions).mockResolvedValue([suggestion()]);
    vi.mocked(api.linkProspect).mockResolvedValue(undefined);
    const onChanged = vi.fn();
    render(<ProspectSuggestionBanner leadId="lead-1" onChanged={onChanged} />);

    const linkButton = await screen.findByRole("button", { name: /Связать/ });
    await act(async () => {
      await user.click(linkButton);
    });

    expect(api.linkProspect).toHaveBeenCalledWith("lead-1", "p-1");
    expect(onChanged).toHaveBeenCalled();
    await waitFor(() => {
      expect(screen.queryByText("Даниил")).not.toBeInTheDocument();
    });
  });

  it("calls dismissProspectSuggestion and removes the item on Отклонить", async () => {
    const user = userEvent.setup();
    vi.mocked(api.getProspectSuggestions).mockResolvedValue([suggestion()]);
    vi.mocked(api.dismissProspectSuggestion).mockResolvedValue(undefined);
    const onChanged = vi.fn();
    render(<ProspectSuggestionBanner leadId="lead-1" onChanged={onChanged} />);

    const dismissButton = await screen.findByRole("button", { name: /Отклонить/ });
    await act(async () => {
      await user.click(dismissButton);
    });

    expect(api.dismissProspectSuggestion).toHaveBeenCalledWith("lead-1", "p-1");
    expect(onChanged).toHaveBeenCalled();
    await waitFor(() => {
      expect(screen.queryByText("Даниил")).not.toBeInTheDocument();
    });
  });
});
