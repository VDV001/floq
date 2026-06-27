import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import type { WebhookEndpoint } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    getWebhooks: vi.fn(),
    getWebhookEventTypes: vi.fn(),
    createWebhook: vi.fn(),
    deleteWebhook: vi.fn(),
    testWebhook: vi.fn(),
    setWebhookActive: vi.fn(),
  },
  ApiError: class ApiError extends Error {
    status: number;
    constructor(message: string, status = 400) {
      super(message);
      this.status = status;
    }
  },
}));

import { api, ApiError } from "@/lib/api";
import { useWebhooks } from "./useWebhooks";

const ep: WebhookEndpoint = { id: "ep-1", url: "https://x.com/a", events: ["lead.created"], active: true };

describe("useWebhooks", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.getWebhooks).mockResolvedValue([ep]);
    vi.mocked(api.getWebhookEventTypes).mockResolvedValue(["lead.created", "lead.qualified"]);
    vi.mocked(api.createWebhook).mockResolvedValue({ ...ep, id: "ep-2" });
    vi.mocked(api.deleteWebhook).mockResolvedValue(undefined);
    vi.mocked(api.testWebhook).mockResolvedValue(undefined);
    vi.mocked(api.setWebhookActive).mockResolvedValue({ ...ep, active: false });
  });

  it("loads endpoints and event types on mount", async () => {
    const { result } = renderHook(() => useWebhooks());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.endpoints).toHaveLength(1);
    expect(result.current.eventTypes).toEqual(["lead.created", "lead.qualified"]);
  });

  it("toggles selected events", async () => {
    const { result } = renderHook(() => useWebhooks());
    await waitFor(() => expect(result.current.loading).toBe(false));
    act(() => result.current.toggleEvent("lead.qualified"));
    expect(result.current.selectedEvents).toContain("lead.qualified");
    act(() => result.current.toggleEvent("lead.qualified"));
    expect(result.current.selectedEvents).not.toContain("lead.qualified");
  });

  it("creates an endpoint, then reloads and resets the form", async () => {
    const { result } = renderHook(() => useWebhooks());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      result.current.setUrl("https://new.com/h");
      result.current.setSecret("supersecretvalue1");
      result.current.toggleEvent("lead.created");
    });
    await act(async () => { await result.current.create(); });

    expect(api.createWebhook).toHaveBeenCalledWith({
      url: "https://new.com/h", events: ["lead.created"], secret: "supersecretvalue1",
    });
    expect(api.getWebhooks).toHaveBeenCalledTimes(2); // initial + reload
    expect(result.current.url).toBe("");
    expect(result.current.secret).toBe("");
    expect(result.current.selectedEvents).toEqual([]);
  });

  it("surfaces a create error and keeps the form", async () => {
    vi.mocked(api.createWebhook).mockRejectedValue(new ApiError("invalid or unsafe URL", 400));
    const { result } = renderHook(() => useWebhooks());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      result.current.setUrl("http://127.0.0.1/x");
      result.current.setSecret("supersecretvalue1");
      result.current.toggleEvent("lead.created");
    });
    await act(async () => { await result.current.create(); });

    expect(result.current.createError).toMatch(/invalid or unsafe URL/);
    expect(result.current.url).toBe("http://127.0.0.1/x"); // form preserved
  });

  it("deletes an endpoint and reloads", async () => {
    const { result } = renderHook(() => useWebhooks());
    await waitFor(() => expect(result.current.loading).toBe(false));
    await act(async () => { await result.current.remove("ep-1"); });
    expect(api.deleteWebhook).toHaveBeenCalledWith("ep-1");
    expect(api.getWebhooks).toHaveBeenCalledTimes(2);
  });

  it("toggles an endpoint's active state and reloads", async () => {
    const { result } = renderHook(() => useWebhooks());
    await waitFor(() => expect(result.current.loading).toBe(false));
    await act(async () => { await result.current.toggleActive("ep-1", false); });
    expect(api.setWebhookActive).toHaveBeenCalledWith("ep-1", false);
    expect(api.getWebhooks).toHaveBeenCalledTimes(2); // initial + reload
    expect(result.current.notice?.ok).toBe(true);
  });

  it("surfaces a failure notice when toggling active fails", async () => {
    vi.mocked(api.setWebhookActive).mockRejectedValue(new ApiError("boom", 500));
    const { result } = renderHook(() => useWebhooks());
    await waitFor(() => expect(result.current.loading).toBe(false));
    await act(async () => { await result.current.toggleActive("ep-1", false); });
    expect(result.current.notice?.ok).toBe(false);
  });

  it("tests an endpoint and sets a success notice", async () => {
    const { result } = renderHook(() => useWebhooks());
    await waitFor(() => expect(result.current.loading).toBe(false));
    await act(async () => { await result.current.test("ep-1"); });
    expect(api.testWebhook).toHaveBeenCalledWith("ep-1");
    expect(result.current.notice?.ok).toBe(true);
  });
});
