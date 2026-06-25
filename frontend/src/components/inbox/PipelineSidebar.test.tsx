import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { PipelineSidebar } from "./PipelineSidebar";
import type { InboxLead } from "./constants";

function inboxLead(over: Partial<InboxLead> = {}): InboxLead {
  return {
    id: "l-1",
    company: "Acme",
    contact: "Иван",
    channel: "telegram",
    preview: "preview",
    timeAgo: "2ч",
    status: "Новый",
    apiStatus: "new",
    pendingRepliesCount: 0,
    ...over,
  };
}

function setup(over: Partial<React.ComponentProps<typeof PipelineSidebar>> = {}) {
  const setActiveStage = vi.fn();
  const setSourceFilter = vi.fn();
  const props = {
    activeStage: "new",
    setActiveStage,
    statusCounts: {},
    leads: [],
    sourceFilter: "",
    setSourceFilter,
    ...over,
  };
  render(<PipelineSidebar {...props} />);
  return { setActiveStage, setSourceFilter };
}

describe("PipelineSidebar", () => {
  it("renders all pipeline stages and fires setActiveStage on click", () => {
    const { setActiveStage } = setup();
    expect(screen.getByText("Новые лиды")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Квалифицированные"));
    expect(setActiveStage).toHaveBeenCalledWith("qualified");
  });

  it("shows the empty AI-summary message when there are no leads", () => {
    setup({ leads: [] });
    expect(
      screen.getByText(/Нет активных лидов/)
    ).toBeInTheDocument();
  });

  it("highlights the followup alert badge when followups exist and stage is inactive", () => {
    setup({
      activeStage: "new",
      statusCounts: { followup: 3 },
      leads: [inboxLead({ apiStatus: "followup", status: "Нужен фоллоуап" })],
    });
    // The followup stage badge shows the count with the alert styling.
    const badge = screen.getByText("3");
    expect(badge).toHaveClass("bg-[#ffdad6]");
  });

  it("summarizes new leads awaiting reply", () => {
    setup({
      statusCounts: { new: 2 },
      leads: [inboxLead(), inboxLead({ id: "l-2" })],
    });
    expect(screen.getByText(/2 новых ожидают ответа/)).toBeInTheDocument();
    expect(screen.getByText(/2 лидов в системе/)).toBeInTheDocument();
  });

  it("summarizes followups when there are no new leads", () => {
    setup({
      statusCounts: { followup: 1 },
      leads: [inboxLead({ apiStatus: "followup" })],
    });
    expect(screen.getByText(/1 требуют фоллоуапа/)).toBeInTheDocument();
    // Singular form for a single lead.
    expect(screen.getByText(/1 лид в системе/)).toBeInTheDocument();
  });

  it("says all leads are in progress when no new and no followup", () => {
    setup({
      statusCounts: { closed: 1 },
      leads: [inboxLead({ apiStatus: "closed" })],
    });
    expect(screen.getByText(/Все лиды в работе/)).toBeInTheDocument();
  });

  it("renders the source filter when leads carry source names and fires setSourceFilter", () => {
    const { setSourceFilter } = setup({
      leads: [
        inboxLead({ sourceName: "2GIS" }),
        inboxLead({ id: "l-2", sourceName: "Сайт" }),
      ],
    });
    expect(screen.getByText("Источник")).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "2GIS" })).toBeInTheDocument();
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "2GIS" } });
    expect(setSourceFilter).toHaveBeenCalledWith("2GIS");
  });

  it("hides the source filter when no leads have source names", () => {
    setup({ leads: [inboxLead()] });
    expect(screen.queryByText("Источник")).not.toBeInTheDocument();
  });
});
