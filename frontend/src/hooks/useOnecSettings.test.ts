import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import type { OnecConfig, OnecMapping } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    getOnecConfig: vi.fn(),
    updateOnecConfig: vi.fn(),
    regenerateOnecWebhook: vi.fn(),
    testOnec: vi.fn(),
    getOnecMapping: vi.fn(),
    updateOnecMapping: vi.fn(),
  },
}));

import { api } from "@/lib/api";
import { useOnecSettings } from "./useOnecSettings";

function cfg(over: Partial<OnecConfig> = {}): OnecConfig {
  return {
    base_url: over.base_url ?? "https://1c.example.com",
    auth_type: over.auth_type ?? "basic",
    auth_secret: over.auth_secret ?? "...e123",
    webhook_secret: over.webhook_secret ?? "...8de2",
    is_active: over.is_active ?? false,
  };
}

function mapping(over: Partial<OnecMapping> = {}): OnecMapping {
  return { rules: over.rules ?? [{ external_type: "Документ.Оплата", kind: "payment", email_field: "email" }] };
}

describe("useOnecSettings", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.getOnecConfig).mockResolvedValue(cfg());
    vi.mocked(api.getOnecMapping).mockResolvedValue(mapping());
    vi.mocked(api.updateOnecConfig).mockImplementation(async () => cfg());
    vi.mocked(api.updateOnecMapping).mockResolvedValue({ saved: true });
    vi.mocked(api.testOnec).mockResolvedValue({ success: true });
    vi.mocked(api.regenerateOnecWebhook).mockResolvedValue({ webhook_secret: "f".repeat(64) });
  });

  it("loads config and mapping on mount", async () => {
    const { result } = renderHook(() => useOnecSettings());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(api.getOnecConfig).toHaveBeenCalledTimes(1);
    expect(api.getOnecMapping).toHaveBeenCalledTimes(1);
    expect(result.current.baseURL).toBe("https://1c.example.com");
    expect(result.current.maskedSecret).toBe("...e123");
    expect(result.current.rules).toHaveLength(1);
  });

  it("does not send a masked secret on save (use-stored)", async () => {
    const { result } = renderHook(() => useOnecSettings());
    await waitFor(() => expect(result.current.loading).toBe(false));

    // The secret input still holds the masked value the user never touched.
    act(() => result.current.setAuthSecret("...e123"));
    await act(async () => {
      await result.current.save();
    });
    const payload = vi.mocked(api.updateOnecConfig).mock.calls[0]![0];
    expect(payload.auth_secret).toBeUndefined();
  });

  it("sends a freshly typed secret on save", async () => {
    const { result } = renderHook(() => useOnecSettings());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => result.current.setAuthSecret("brand-new-secret"));
    await act(async () => {
      await result.current.save();
    });
    const payload = vi.mocked(api.updateOnecConfig).mock.calls[0]![0];
    expect(payload.auth_secret).toBe("brand-new-secret");
  });

  it("exposes the full webhook secret once after regenerate", async () => {
    const { result } = renderHook(() => useOnecSettings());
    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.regenerateWebhook();
    });
    expect(result.current.fullWebhook).toBe("f".repeat(64));
  });

  it("adds, edits and removes mapping rules then saves them", async () => {
    const { result } = renderHook(() => useOnecSettings());
    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => result.current.addRule());
    expect(result.current.rules).toHaveLength(2);

    await act(async () => result.current.updateRule(1, { external_type: "Документ.Заказ", kind: "order_status" }));
    expect(result.current.rules[1].external_type).toBe("Документ.Заказ");

    await act(async () => result.current.removeRule(0));
    expect(result.current.rules).toHaveLength(1);

    await act(async () => {
      await result.current.saveMapping();
    });
    expect(api.updateOnecMapping).toHaveBeenCalledWith(result.current.rules);
  });

  it("runs a connection test with the current form values", async () => {
    const { result } = renderHook(() => useOnecSettings());
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => result.current.setBaseURL("https://new.example.com"));
    await act(async () => {
      await result.current.test();
    });
    const payload = vi.mocked(api.testOnec).mock.calls[0]![0];
    expect(payload.base_url).toBe("https://new.example.com");
    expect(result.current.testResult?.success).toBe(true);
  });
});
