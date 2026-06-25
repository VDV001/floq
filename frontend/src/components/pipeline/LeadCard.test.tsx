import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { LeadCard } from "./LeadCard";
import type { PipelineLead } from "./constants";

const getQualification = vi.fn();
vi.mock("@/lib/api", () => ({
  api: { getQualification: (...a: unknown[]) => getQualification(...a) },
}));
vi.mock("./ChannelBadge", () => ({
  ChannelBadge: ({ channel }: { channel: string }) => <span>badge:{channel}</span>,
}));

function lead(over: Partial<PipelineLead> = {}): PipelineLead {
  return {
    id: "l1",
    name: "Alice",
    company: "Acme",
    channel: "telegram",
    timeAgo: "2ч назад",
    ...over,
  } as PipelineLead;
}

const qual = {
  identified_need: "нужен CRM",
  estimated_budget: "$5k",
  deadline: "Q3",
  score: 8,
  score_reason: "горячий",
  recommended_action: "позвонить",
};

beforeEach(() => getQualification.mockReset().mockResolvedValue(qual));

describe("pipeline/LeadCard", () => {
  it("renders the collapsed card with name and company", () => {
    render(<LeadCard lead={lead()} />);
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("Acme")).toBeInTheDocument();
    expect(screen.queryByText("Скор квалификации")).not.toBeInTheDocument();
  });

  it("omits the company line when there is none", () => {
    render(<LeadCard lead={lead({ company: "" })} />);
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.queryByText("Acme")).not.toBeInTheDocument();
  });

  it("opens the detail and loads the qualification", async () => {
    render(<LeadCard lead={lead({ id: "l9" })} />);
    await userEvent.click(screen.getByText("Alice"));

    await waitFor(() => expect(getQualification).toHaveBeenCalledWith("l9"));
    expect(await screen.findByText("Скор квалификации")).toBeInTheDocument();
    expect(screen.getByText("нужен CRM")).toBeInTheDocument();
    expect(screen.getByText("$5k")).toBeInTheDocument();
    expect(screen.getByText("позвонить")).toBeInTheDocument();
    expect(screen.getByText("8")).toBeInTheDocument();
    // The "open lead" deep link is present.
    expect(screen.getByText("Открыть лида").closest("a")).toHaveAttribute("href", "/inbox/l9");
  });

  it("shows the empty state when qualification is missing", async () => {
    getQualification.mockResolvedValue(null);
    render(<LeadCard lead={lead()} />);
    await userEvent.click(screen.getByText("Alice"));
    expect(await screen.findByText("Нет данных квалификации")).toBeInTheDocument();
  });

  it("closes the detail via the close button", async () => {
    render(<LeadCard lead={lead()} />);
    await userEvent.click(screen.getByText("Alice"));
    expect(await screen.findByText("Скор квалификации")).toBeInTheDocument();

    // Close button is the only button in the open panel.
    await userEvent.click(screen.getByRole("button"));
    await waitFor(() =>
      expect(screen.queryByText("Скор квалификации")).not.toBeInTheDocument(),
    );
  });
});
