import { render, screen, fireEvent, within } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { WebhookSection } from "./WebhookSection";
import type { WebhookEndpoint } from "@/lib/api";

type Props = React.ComponentProps<typeof WebhookSection>;

const endpoints: WebhookEndpoint[] = [
  { id: "ep-1", url: "https://hooks.example.com/a", events: ["lead.created"], active: true },
];

function setup(over: Partial<Props> = {}) {
  const props: Props = {
    endpoints,
    eventTypes: ["lead.created", "lead.qualified", "lead.archived"],
    loading: false,
    url: "",
    setUrl: vi.fn(),
    secret: "",
    setSecret: vi.fn(),
    selectedEvents: [],
    toggleEvent: vi.fn(),
    creating: false,
    createError: null,
    onCreate: vi.fn(),
    onDelete: vi.fn(),
    onTest: vi.fn(),
    onToggleActive: vi.fn(),
    togglingId: null,
    testingId: null,
    notice: null,
    ...over,
  };
  render(<WebhookSection {...props} />);
  return props;
}

describe("WebhookSection", () => {
  it("lists existing endpoints with their URL and events", () => {
    setup();
    // Scope to the endpoint list item: "lead.created" also appears as a form
    // checkbox label, so a global query would be ambiguous.
    const item = screen.getByRole("listitem");
    expect(within(item).getByText("https://hooks.example.com/a")).toBeInTheDocument();
    expect(within(item).getByText("lead.created")).toBeInTheDocument();
  });

  it("renders an event-type checkbox for each available event", () => {
    setup();
    expect(screen.getByRole("checkbox", { name: /lead\.qualified/ })).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: /lead\.archived/ })).toBeInTheDocument();
  });

  it("propagates url and secret edits", () => {
    const props = setup();
    fireEvent.change(screen.getByPlaceholderText(/https:\/\//), { target: { value: "https://x.com/h" } });
    expect(props.setUrl).toHaveBeenCalledWith("https://x.com/h");
    fireEvent.change(screen.getByPlaceholderText(/секрет/i), { target: { value: "mysecret123456" } });
    expect(props.setSecret).toHaveBeenCalledWith("mysecret123456");
  });

  it("toggles an event when its checkbox is clicked", () => {
    const props = setup();
    fireEvent.click(screen.getByRole("checkbox", { name: /lead\.qualified/ }));
    expect(props.toggleEvent).toHaveBeenCalledWith("lead.qualified");
  });

  it("disables the add button until url, secret and at least one event are set", () => {
    setup({ url: "", secret: "", selectedEvents: [] });
    expect(screen.getByRole("button", { name: /добавить/i })).toBeDisabled();
  });

  it("enables and fires onCreate when the form is complete", () => {
    const props = setup({ url: "https://x.com/h", secret: "supersecretvalue1", selectedEvents: ["lead.created"] });
    const btn = screen.getByRole("button", { name: /добавить/i });
    expect(btn).not.toBeDisabled();
    fireEvent.click(btn);
    expect(props.onCreate).toHaveBeenCalled();
  });

  it("fires onTest and onDelete for an endpoint", () => {
    const props = setup();
    fireEvent.click(screen.getByRole("button", { name: /проверить/i }));
    expect(props.onTest).toHaveBeenCalledWith("ep-1");
    fireEvent.click(screen.getByRole("button", { name: /удалить/i }));
    expect(props.onDelete).toHaveBeenCalledWith("ep-1");
  });

  it("shows an active/inactive badge per endpoint", () => {
    setup({
      endpoints: [
        { id: "ep-1", url: "https://a.com/h", events: ["lead.created"], active: true },
        { id: "ep-2", url: "https://b.com/h", events: ["lead.created"], active: false },
      ],
    });
    const items = screen.getAllByRole("listitem");
    expect(within(items[0]).getByText(/активен/i)).toBeInTheDocument();
    expect(within(items[1]).getByText(/отключён/i)).toBeInTheDocument();
  });

  it("fires onToggleActive to disable an active endpoint", () => {
    const props = setup(); // ep-1 is active
    fireEvent.click(screen.getByRole("button", { name: /отключить/i }));
    expect(props.onToggleActive).toHaveBeenCalledWith("ep-1", false);
  });

  it("fires onToggleActive to enable an inactive endpoint", () => {
    const props = setup({
      endpoints: [{ id: "ep-9", url: "https://c.com/h", events: ["lead.created"], active: false }],
    });
    fireEvent.click(screen.getByRole("button", { name: /включить/i }));
    expect(props.onToggleActive).toHaveBeenCalledWith("ep-9", true);
  });

  it("surfaces a create error", () => {
    setup({ createError: "invalid or unsafe URL" });
    expect(screen.getByText(/invalid or unsafe URL/)).toBeInTheDocument();
  });
});
