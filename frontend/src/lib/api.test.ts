import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// We need to import the module after mocking globals
let api: typeof import("./api")["api"];

// Helper to create a mock Response
function mockResponse(body: unknown, init?: { status?: number; statusText?: string; headers?: Record<string, string> }) {
  const status = init?.status ?? 200;
  const statusText = init?.statusText ?? "OK";
  const headers = new Headers(init?.headers);
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText,
    headers,
    json: vi.fn().mockResolvedValue(body),
    blob: vi.fn().mockResolvedValue(new Blob(["test"], { type: "text/csv" })),
  } as unknown as Response;
}

describe("api module", () => {
  let fetchMock: ReturnType<typeof vi.fn>;
  let localStorageData: Record<string, string>;

  beforeEach(async () => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    localStorageData = {};
    const localStorageMock = {
      getItem: vi.fn((key: string) => localStorageData[key] ?? null),
      setItem: vi.fn((key: string, value: string) => {
        localStorageData[key] = value;
      }),
      removeItem: vi.fn((key: string) => {
        delete localStorageData[key];
      }),
    };
    vi.stubGlobal("localStorage", localStorageMock);

    // Re-import module fresh each time
    vi.resetModules();
    const mod = await import("./api");
    api = mod.api;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // ── apiFetch basics ──────────────────────────────────────────────

  describe("apiFetch", () => {
    it("makes GET request and returns JSON (happy path)", async () => {
      const data = [{ id: "1", name: "Test Lead" }];
      fetchMock.mockResolvedValueOnce(mockResponse(data));

      const result = await api.getLeads();

      expect(fetchMock).toHaveBeenCalledOnce();
      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/leads");
      expect(opts.headers["Content-Type"]).toBe("application/json");
      expect(result).toEqual(data);
    });

    it("attaches Authorization header when token exists", async () => {
      localStorageData["token"] = "my-jwt-token";
      fetchMock.mockResolvedValueOnce(mockResponse([]));

      await api.getLeads();

      const [, opts] = fetchMock.mock.calls[0];
      expect(opts.headers["Authorization"]).toBe("Bearer my-jwt-token");
    });

    it("omits Authorization header when no token", async () => {
      fetchMock.mockResolvedValueOnce(mockResponse([]));

      await api.getLeads();

      const [, opts] = fetchMock.mock.calls[0];
      expect(opts.headers["Authorization"]).toBeUndefined();
    });

    it("throws on non-ok response (400)", async () => {
      fetchMock.mockResolvedValueOnce(
        mockResponse({ error: "bad" }, { status: 400, statusText: "Bad Request" })
      );

      await expect(api.getLeads()).rejects.toThrow("API error: 400 Bad Request");
    });

    it("throws on server error (500)", async () => {
      fetchMock.mockResolvedValueOnce(
        mockResponse(null, { status: 500, statusText: "Internal Server Error" })
      );

      await expect(api.getLeads()).rejects.toThrow("API error: 500 Internal Server Error");
    });

    it("returns undefined on 204 No Content without calling json()", async () => {
      // A real fetch Response with empty body throws SyntaxError when
      // .json() is called. The mock pins the same shape by rejecting
      // json() with the same error the browser would surface — any
      // implementation that unconditionally invokes res.json() on a
      // 204 fails this test, matching the real-world bug operators
      // would hit on approve/reject (HTTP 204) in production.
      const jsonMock = vi.fn().mockRejectedValue(new SyntaxError("Unexpected end of JSON input"));
      fetchMock.mockResolvedValueOnce({
        ok: true,
        status: 204,
        statusText: "No Content",
        headers: new Headers(),
        json: jsonMock,
      } as unknown as Response);

      const result = await api.approvePendingReply("pr-1");

      expect(result).toBeUndefined();
      expect(jsonMock).not.toHaveBeenCalled();
    });
  });

  // ── 401 refresh token flow ───────────────────────────────────────

  describe("401 refresh token flow", () => {
    it("refreshes token and retries on 401", async () => {
      localStorageData["token"] = "expired-token";
      localStorageData["refresh_token"] = "my-refresh";

      // First call → 401
      fetchMock.mockResolvedValueOnce(
        mockResponse(null, { status: 401, statusText: "Unauthorized" })
      );
      // Refresh call → success
      fetchMock.mockResolvedValueOnce(
        mockResponse({ token: "new-token", refresh_token: "new-refresh" })
      );
      // Retry call → success
      const retryData = [{ id: "1" }];
      fetchMock.mockResolvedValueOnce(mockResponse(retryData));

      const result = await api.getLeads();

      expect(fetchMock).toHaveBeenCalledTimes(3);
      // Verify refresh was called correctly
      const [refreshUrl, refreshOpts] = fetchMock.mock.calls[1];
      expect(refreshUrl).toBe("http://localhost:8080/api/auth/refresh");
      expect(JSON.parse(refreshOpts.body)).toEqual({ refresh_token: "my-refresh" });
      // Verify retry used new token
      const [, retryOpts] = fetchMock.mock.calls[2];
      expect(retryOpts.headers["Authorization"]).toBe("Bearer new-token");
      expect(result).toEqual(retryData);
    });

    it("redirects to /login when refresh fails", async () => {
      localStorageData["token"] = "expired-token";
      localStorageData["refresh_token"] = "bad-refresh";

      // Mock window.location
      const locationMock = { href: "" };
      vi.stubGlobal("location", locationMock);

      // First call → 401
      fetchMock.mockResolvedValueOnce(
        mockResponse(null, { status: 401, statusText: "Unauthorized" })
      );
      // Refresh call → fails
      fetchMock.mockResolvedValueOnce(
        mockResponse(null, { status: 401, statusText: "Unauthorized" })
      );

      // After redirect, the function throws because original res is not ok
      await expect(api.getLeads()).rejects.toThrow("API error: 401");
      expect(locationMock.href).toBe("/login");
    });

    it("redirects to /login when no refresh token and 401", async () => {
      localStorageData["token"] = "expired-token";
      // no refresh_token

      fetchMock.mockResolvedValueOnce(
        mockResponse(null, { status: 401, statusText: "Unauthorized" })
      );

      // No refresh token → no refresh attempt, just throws
      await expect(api.getLeads()).rejects.toThrow("API error: 401");
      expect(fetchMock).toHaveBeenCalledOnce();
    });
  });

  // ── apiDownload ──────────────────────────────────────────────────

  describe("apiDownload", () => {
    it("creates blob download link with filename from Content-Disposition", async () => {
      const blobUrl = "blob:http://localhost/abc";
      vi.stubGlobal("URL", {
        createObjectURL: vi.fn(() => blobUrl),
        revokeObjectURL: vi.fn(),
      });

      const mockA = {
        href: "",
        download: "",
        click: vi.fn(),
        remove: vi.fn(),
      };
      vi.spyOn(document, "createElement").mockReturnValue(mockA as unknown as HTMLElement);
      vi.spyOn(document.body, "appendChild").mockImplementation((node) => node);

      fetchMock.mockResolvedValueOnce(
        mockResponse(null, {
          status: 200,
          headers: { "Content-Disposition": 'attachment; filename="leads.csv"' },
        })
      );

      await api.exportLeadsCSV();

      expect(mockA.href).toBe(blobUrl);
      expect(mockA.download).toBe("leads.csv");
      expect(mockA.click).toHaveBeenCalledOnce();
      expect(mockA.remove).toHaveBeenCalledOnce();
    });

    it("uses default filename when Content-Disposition is missing", async () => {
      const blobUrl = "blob:http://localhost/abc";
      vi.stubGlobal("URL", {
        createObjectURL: vi.fn(() => blobUrl),
        revokeObjectURL: vi.fn(),
      });

      const mockA = {
        href: "",
        download: "",
        click: vi.fn(),
        remove: vi.fn(),
      };
      vi.spyOn(document, "createElement").mockReturnValue(mockA as unknown as HTMLElement);
      vi.spyOn(document.body, "appendChild").mockImplementation((node) => node);

      fetchMock.mockResolvedValueOnce(mockResponse(null, { status: 200 }));

      await api.exportLeadsCSV();

      expect(mockA.download).toBe("export.csv");
    });

    it("throws on non-ok download response", async () => {
      fetchMock.mockResolvedValueOnce(
        mockResponse(null, { status: 404, statusText: "Not Found" })
      );

      await expect(api.exportLeadsCSV()).rejects.toThrow("Download error: 404");
    });
  });

  // ── apiUploadFile ────────────────────────────────────────────────

  describe("apiUploadFile", () => {
    it("sends FormData with file via POST", async () => {
      const file = new File(["csv-data"], "contacts.csv", { type: "text/csv" });
      fetchMock.mockResolvedValueOnce(mockResponse({ imported: 5 }));

      const result = await api.importLeadsCSV(file);

      expect(result).toEqual({ imported: 5 });
      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/leads/import");
      expect(opts.method).toBe("POST");
      expect(opts.body).toBeInstanceOf(FormData);
      expect((opts.body as FormData).get("file")).toBe(file);
    });

    it("does not set Content-Type (browser sets multipart boundary)", async () => {
      const file = new File(["data"], "test.csv");
      fetchMock.mockResolvedValueOnce(mockResponse({ imported: 1 }));

      await api.importLeadsCSV(file);

      const [, opts] = fetchMock.mock.calls[0];
      // apiUploadFile should NOT set Content-Type so the browser adds multipart boundary
      expect(opts.headers?.["Content-Type"]).toBeUndefined();
    });

    it("throws on upload error", async () => {
      const file = new File(["data"], "test.csv");
      fetchMock.mockResolvedValueOnce(
        mockResponse(null, { status: 413, statusText: "Payload Too Large" })
      );

      await expect(api.importLeadsCSV(file)).rejects.toThrow("Upload error: 413");
    });
  });

  // ── Prospect import/export endpoints ─────────────────────────────
  // The three prospect file endpoints are thin wrappers over the
  // apiUploadFile / apiDownload helpers tested above; covering them
  // pins their URL contract and closes the last functions in api.ts.

  describe("prospect import/export endpoints", () => {
    it("importProspectsCSV → POST /api/prospects/import with FormData", async () => {
      const file = new File(["row"], "p.csv", { type: "text/csv" });
      fetchMock.mockResolvedValueOnce(mockResponse({ imported: 3 }));

      const result = await api.importProspectsCSV(file);

      expect(result).toEqual({ imported: 3 });
      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/prospects/import");
      expect(opts.method).toBe("POST");
      expect(opts.body).toBeInstanceOf(FormData);
    });

    it.each([
      { name: "exportProspectsCSV", run: () => api.exportProspectsCSV(), url: "http://localhost:8080/api/prospects/export" },
      { name: "downloadProspectTemplate", run: () => api.downloadProspectTemplate(), url: "http://localhost:8080/api/prospects/template" },
    ])("$name triggers a blob download from $url", async ({ run, url }) => {
      vi.stubGlobal("URL", {
        createObjectURL: vi.fn(() => "blob:http://localhost/x"),
        revokeObjectURL: vi.fn(),
      });
      const mockA = { href: "", download: "", click: vi.fn(), remove: vi.fn() };
      vi.spyOn(document, "createElement").mockReturnValue(mockA as unknown as HTMLElement);
      vi.spyOn(document.body, "appendChild").mockImplementation((node) => node);
      fetchMock.mockResolvedValueOnce(mockResponse(null, { status: 200 }));

      await run();

      expect(fetchMock.mock.calls[0][0]).toBe(url);
      expect(mockA.click).toHaveBeenCalledOnce();
    });
  });

  // ── Specific API methods (URL + method verification) ─────────────

  describe("API method contracts", () => {
    beforeEach(() => {
      fetchMock.mockResolvedValue(mockResponse({}));
    });

    it("getLeads → GET /api/leads", async () => {
      await api.getLeads();
      expect(fetchMock.mock.calls[0][0]).toBe("http://localhost:8080/api/leads");
      expect(fetchMock.mock.calls[0][1].method).toBeUndefined(); // GET by default
    });

    it("createProspect → POST /api/prospects with body", async () => {
      const data = { name: "John", email: "john@test.com", company: "Acme" };
      await api.createProspect(data);
      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/prospects");
      expect(opts.method).toBe("POST");
      expect(JSON.parse(opts.body)).toEqual(data);
    });

    it("setProspectConsent → POST /api/prospects/:id/consent with status", async () => {
      fetchMock.mockResolvedValue(mockResponse({ consent_status: "obtained" }));
      await api.setProspectConsent("prospect-1", "obtained");
      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/prospects/prospect-1/consent");
      expect(opts.method).toBe("POST");
      expect(JSON.parse(opts.body)).toEqual({ status: "obtained" });
    });

    it("getSourceStats → GET /api/sources/stats", async () => {
      await api.getSourceStats();
      expect(fetchMock.mock.calls[0][0]).toBe("http://localhost:8080/api/sources/stats");
    });

    it("updateLeadStatus → PATCH /api/leads/:id/status", async () => {
      await api.updateLeadStatus("lead-1", "qualified");
      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/leads/lead-1/status");
      expect(opts.method).toBe("PATCH");
      expect(JSON.parse(opts.body)).toEqual({ status: "qualified" });
    });

    it("deleteProspect → DELETE /api/prospects/:id", async () => {
      await api.deleteProspect("p-123");
      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/prospects/p-123");
      expect(opts.method).toBe("DELETE");
    });

    it("login → POST /api/auth/login with credentials", async () => {
      await api.login("user@test.com", "secret");
      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/auth/login");
      expect(opts.method).toBe("POST");
      expect(JSON.parse(opts.body)).toEqual({ email: "user@test.com", password: "secret" });
    });

    it("getSources → GET /api/sources", async () => {
      await api.getSources();
      expect(fetchMock.mock.calls[0][0]).toBe("http://localhost:8080/api/sources");
    });

    it("qualifyLead → POST /api/leads/:id/qualify", async () => {
      await api.qualifyLead("lead-2");
      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/leads/lead-2/qualify");
      expect(opts.method).toBe("POST");
    });

    it("getPendingReplies → GET /api/leads/:id/pending-replies", async () => {
      await api.getPendingReplies("lead-7");
      expect(fetchMock.mock.calls[0][0]).toBe("http://localhost:8080/api/leads/lead-7/pending-replies");
      expect(fetchMock.mock.calls[0][1].method).toBeUndefined();
    });

    it("approvePendingReply → POST /api/pending-replies/:id/approve", async () => {
      await api.approvePendingReply("pr-1");
      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/pending-replies/pr-1/approve");
      expect(opts.method).toBe("POST");
    });

    it("rejectPendingReply → POST /api/pending-replies/:id/reject", async () => {
      await api.rejectPendingReply("pr-2");
      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/pending-replies/pr-2/reject");
      expect(opts.method).toBe("POST");
    });

    it("bulkPendingReplies → POST /api/pending-replies/bulk with ids and decision", async () => {
      const wireResponse = {
        results: [
          { id: "pr-1", ok: true },
          { id: "pr-2", ok: false, error: "not found" },
        ],
      };
      fetchMock.mockResolvedValueOnce(mockResponse(wireResponse));

      const result = await api.bulkPendingReplies({
        ids: ["pr-1", "pr-2"],
        decision: "approve",
      });

      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/pending-replies/bulk");
      expect(opts.method).toBe("POST");
      expect(JSON.parse(opts.body)).toEqual({
        ids: ["pr-1", "pr-2"],
        decision: "approve",
      });
      expect(result).toEqual(wireResponse);
    });

    it("listPendingReplies → GET /api/pending-replies?status=pending with joined lead snippet", async () => {
      const sample = [
        {
          id: "pr-1",
          lead_id: "lead-1",
          channel: "telegram",
          kind: "booking_link",
          body: "queue body",
          status: "pending",
          created_at: "2026-05-20T10:00:00Z",
          lead: {
            contact_name: "Иван Петров",
            company: "ACME",
            channel: "telegram",
            telegram_chat_id: 987654,
          },
        },
      ];
      fetchMock.mockResolvedValueOnce(mockResponse(sample));

      const result = await api.listPendingReplies();

      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/pending-replies?status=pending");
      expect(opts.method).toBeUndefined();
      expect(result).toEqual(sample);
    });

    it("updatePendingReply → PATCH /api/pending-replies/:id with body", async () => {
      const updated = {
        id: "pr-3",
        lead_id: "lead-3",
        channel: "telegram",
        kind: "booking_link",
        body: "edited body",
        status: "pending",
        created_at: "2026-05-19T10:00:00Z",
      };
      fetchMock.mockResolvedValueOnce(mockResponse(updated));

      const result = await api.updatePendingReply("pr-3", "edited body");

      const [url, opts] = fetchMock.mock.calls[0];
      expect(url).toBe("http://localhost:8080/api/pending-replies/pr-3");
      expect(opts.method).toBe("PATCH");
      expect(JSON.parse(opts.body)).toEqual({ body: "edited body" });
      // PATCH returns 200 + the updated entity (NOT 204), so the API
      // method must surface the new body to the UI without a refetch.
      expect(result).toEqual(updated);
    });
  });

  // ── Exhaustive endpoint contracts ────────────────────────────────
  // One row per remaining endpoint: URL + HTTP verb + request body. This
  // pins the wire contract the focused tests above leave untouched — a
  // typo in a path, a wrong method, or a mis-named snake_case field
  // (full_name / prospect_id / code_hash / use_stored / send_now /
  // is_active) is exactly what a row catches — and it also exercises the
  // default-argument branches (sendNow, chat context, useStored, the
  // analytics periods and the getHotLeads query builder).
  describe("endpoint contracts", () => {
    const BASE = "http://localhost:8080";

    beforeEach(() => {
      fetchMock.mockResolvedValue(mockResponse({}));
    });

    type Case = {
      name: string;
      run: () => Promise<unknown>;
      url: string;
      method?: string;
      body?: unknown;
    };

    const cases: Case[] = [
      // Auth
      { name: "register", run: () => api.register("e@x.com", "pw", "Jane Doe"), url: "/api/auth/register", method: "POST", body: { email: "e@x.com", password: "pw", full_name: "Jane Doe" } },
      { name: "refresh", run: () => api.refresh("rt"), url: "/api/auth/refresh", method: "POST", body: { refresh_token: "rt" } },
      // Leads
      { name: "getLead", run: () => api.getLead("l1"), url: "/api/leads/l1" },
      { name: "getProspectSuggestions", run: () => api.getProspectSuggestions("l1"), url: "/api/leads/l1/prospect-suggestions" },
      { name: "linkProspect", run: () => api.linkProspect("l1", "p1"), url: "/api/leads/l1/link-prospect", method: "POST", body: { prospect_id: "p1" } },
      { name: "dismissProspectSuggestion", run: () => api.dismissProspectSuggestion("l1", "p1"), url: "/api/leads/l1/dismiss-prospect-suggestion", method: "POST", body: { prospect_id: "p1" } },
      { name: "getSuggestionCounts", run: () => api.getSuggestionCounts(), url: "/api/leads/suggestion-counts" },
      { name: "getMessages", run: () => api.getMessages("l1"), url: "/api/leads/l1/messages" },
      { name: "getMessages aggregated", run: () => api.getMessages("l1", { aggregated: true }), url: "/api/leads/l1/messages?aggregated=true" },
      { name: "sendMessage", run: () => api.sendMessage("l1", "hi"), url: "/api/leads/l1/send", method: "POST", body: { body: "hi" } },
      { name: "getQualification", run: () => api.getQualification("l1"), url: "/api/leads/l1/qualification" },
      { name: "getDraft", run: () => api.getDraft("l1"), url: "/api/leads/l1/draft" },
      { name: "regenerateDraft", run: () => api.regenerateDraft("l1"), url: "/api/leads/l1/draft/regen", method: "POST" },
      // Reminders
      { name: "getReminders", run: () => api.getReminders(), url: "/api/reminders" },
      { name: "snoozeReminder", run: () => api.snoozeReminder("r1"), url: "/api/reminders/r1/snooze", method: "POST" },
      { name: "dismissReminder", run: () => api.dismissReminder("r1"), url: "/api/reminders/r1/dismiss", method: "POST" },
      // Sources
      { name: "createSourceCategory", run: () => api.createSourceCategory("Cat"), url: "/api/sources/categories", method: "POST", body: { name: "Cat" } },
      { name: "createSource", run: () => api.createSource("c1", "Src"), url: "/api/sources", method: "POST", body: { category_id: "c1", name: "Src" } },
      // Prospects
      { name: "getProspects", run: () => api.getProspects(), url: "/api/prospects" },
      { name: "getVerifyStatus", run: () => api.getVerifyStatus("p1"), url: "/api/prospects/p1/verify" },
      // Verification
      { name: "verifyEmail", run: () => api.verifyEmail("e@x.com"), url: "/api/verify/email", method: "POST", body: { email: "e@x.com" } },
      { name: "verifyBatch", run: () => api.verifyBatch(), url: "/api/verify/batch", method: "POST" },
      // Parser
      { name: "scrapeWebsite", run: () => api.scrapeWebsite("http://x"), url: "/api/parser/website", method: "POST", body: { url: "http://x" } },
      { name: "searchTwoGIS", run: () => api.searchTwoGIS("q", "Moscow"), url: "/api/parser/twogis", method: "POST", body: { query: "q", city: "Moscow" } },
      // Sequences
      { name: "getSequences", run: () => api.getSequences(), url: "/api/sequences" },
      { name: "getSequence", run: () => api.getSequence("s1"), url: "/api/sequences/s1" },
      { name: "createSequence", run: () => api.createSequence("Seq"), url: "/api/sequences", method: "POST", body: { name: "Seq" } },
      { name: "updateSequence", run: () => api.updateSequence("s1", "New"), url: "/api/sequences/s1", method: "PUT", body: { name: "New" } },
      { name: "deleteSequence", run: () => api.deleteSequence("s1"), url: "/api/sequences/s1", method: "DELETE" },
      { name: "addStep", run: () => api.addStep("s1", { step_order: 1, delay_days: 2, channel: "email", prompt_hint: "h" }), url: "/api/sequences/s1/steps", method: "POST", body: { step_order: 1, delay_days: 2, channel: "email", prompt_hint: "h" } },
      { name: "deleteStep", run: () => api.deleteStep("s1", "st1"), url: "/api/sequences/s1/steps/st1", method: "DELETE" },
      { name: "previewMessage", run: () => api.previewMessage("n", "c", "ctx", "email", "h"), url: "/api/sequences/preview", method: "POST", body: { name: "n", company: "c", context: "ctx", channel: "email", hint: "h" } },
      { name: "launchSequence default sendNow=true", run: () => api.launchSequence("s1", ["p1"]), url: "/api/sequences/s1/launch", method: "POST", body: { prospect_ids: ["p1"], send_now: true } },
      { name: "launchSequence sendNow=false", run: () => api.launchSequence("s1", ["p1"], false), url: "/api/sequences/s1/launch", method: "POST", body: { prospect_ids: ["p1"], send_now: false } },
      { name: "toggleSequence", run: () => api.toggleSequence("s1", true), url: "/api/sequences/s1/toggle", method: "PATCH", body: { is_active: true } },
      // Outbound
      { name: "getOutboundQueue", run: () => api.getOutboundQueue(), url: "/api/outbound/queue" },
      { name: "approveMessage", run: () => api.approveMessage("m1"), url: "/api/outbound/m1/approve", method: "POST" },
      { name: "rejectMessage", run: () => api.rejectMessage("m1"), url: "/api/outbound/m1/reject", method: "POST" },
      { name: "editMessage", run: () => api.editMessage("m1", "b"), url: "/api/outbound/m1/edit", method: "POST", body: { body: "b" } },
      { name: "getOutboundSent", run: () => api.getOutboundSent(), url: "/api/outbound/sent" },
      { name: "getOutboundStats", run: () => api.getOutboundStats(), url: "/api/outbound/stats" },
      // AI chat (default context branch)
      { name: "chatWithAI with context", run: () => api.chatWithAI("msg", [{ role: "user", content: "hi" }], "ctx"), url: "/api/chat", method: "POST", body: { message: "msg", history: [{ role: "user", content: "hi" }], context: "ctx" } },
      { name: "chatWithAI default context", run: () => api.chatWithAI("msg", []), url: "/api/chat", method: "POST", body: { message: "msg", history: [], context: "" } },
      // Telegram account
      { name: "tgAccountSendCode", run: () => api.tgAccountSendCode("+123"), url: "/api/telegram-account/send-code", method: "POST", body: { phone: "+123" } },
      { name: "tgAccountVerify", run: () => api.tgAccountVerify("+1", "123", "hash"), url: "/api/telegram-account/verify", method: "POST", body: { phone: "+1", code: "123", code_hash: "hash" } },
      { name: "tgAccountStatus", run: () => api.tgAccountStatus(), url: "/api/telegram-account/status" },
      { name: "tgAccountDisconnect", run: () => api.tgAccountDisconnect(), url: "/api/telegram-account", method: "DELETE" },
      // Usage / Settings (use_stored branches)
      { name: "getUsage", run: () => api.getUsage(), url: "/api/usage" },
      { name: "getSettings", run: () => api.getSettings(), url: "/api/settings" },
      { name: "updateSettings", run: () => api.updateSettings({}), url: "/api/settings", method: "PUT", body: {} },
      { name: "testIMAP default useStored", run: () => api.testIMAP("h", "993", "u", "pw"), url: "/api/settings/test-imap", method: "POST", body: { host: "h", port: "993", user: "u", password: "pw", use_stored: false } },
      { name: "testIMAP useStored=true", run: () => api.testIMAP("h", "993", "u", "pw", true), url: "/api/settings/test-imap", method: "POST", body: { host: "h", port: "993", user: "u", password: "pw", use_stored: true } },
      { name: "testAI", run: () => api.testAI("openai", "gpt", "key"), url: "/api/settings/test-ai", method: "POST", body: { provider: "openai", model: "gpt", api_key: "key", use_stored: false } },
      { name: "testSMTP", run: () => api.testSMTP("h", "587", "u", "pw"), url: "/api/settings/test-smtp", method: "POST", body: { host: "h", port: "587", user: "u", password: "pw" } },
      { name: "testResend", run: () => api.testResend("key"), url: "/api/settings/test-resend", method: "POST", body: { api_key: "key", use_stored: false } },
      // Analytics (default-period branches + query builder)
      { name: "getSequenceAnalytics default", run: () => api.getSequenceAnalytics(), url: "/api/analytics/sequences?period=all" },
      { name: "getSequenceAnalytics week", run: () => api.getSequenceAnalytics("week"), url: "/api/analytics/sequences?period=week" },
      { name: "getCostRatios default", run: () => api.getCostRatios(), url: "/api/analytics/cost-ratios?period=month" },
      { name: "getInboxAnalytics default", run: () => api.getInboxAnalytics(), url: "/api/analytics/inbox?period=month" },
      { name: "getHotLeads no params", run: () => api.getHotLeads(), url: "/api/analytics/hot-leads" },
      { name: "getHotLeads all params", run: () => api.getHotLeads({ period: "week", status: "new", channel: "email", limit: 5 }), url: "/api/analytics/hot-leads?period=week&status=new&channel=email&limit=5" },
      // 1C
      { name: "getOnecConfig", run: () => api.getOnecConfig(), url: "/api/onec/config" },
      { name: "updateOnecConfig", run: () => api.updateOnecConfig({ base_url: "https://1c", is_active: true }), url: "/api/onec/config", method: "PUT", body: { base_url: "https://1c", is_active: true } },
      { name: "regenerateOnecWebhook", run: () => api.regenerateOnecWebhook(), url: "/api/onec/config/regenerate-webhook", method: "POST" },
      { name: "testOnec", run: () => api.testOnec({ base_url: "https://1c" }), url: "/api/onec/test", method: "POST", body: { base_url: "https://1c" } },
      { name: "getOnecMapping", run: () => api.getOnecMapping(), url: "/api/onec/mapping" },
      { name: "updateOnecMapping", run: () => api.updateOnecMapping([{ external_type: "X", kind: "payment", email_field: "email" }]), url: "/api/onec/mapping", method: "PUT", body: { rules: [{ external_type: "X", kind: "payment", email_field: "email" }] } },
      { name: "getCostSummary", run: () => api.getCostSummary("2026-01-01", "2026-02-01"), url: "/api/audit/cost-summary?from=2026-01-01&to=2026-02-01" },
    ];

    it.each(cases)("$name → $url", async (c) => {
      await c.run();
      const calls = fetchMock.mock.calls;
      const [url, opts] = calls[calls.length - 1];
      expect(url).toBe(BASE + c.url);
      if (c.method) {
        expect(opts?.method).toBe(c.method);
      } else {
        expect(opts?.method).toBeUndefined();
      }
      if (c.body !== undefined) {
        expect(JSON.parse(opts.body)).toEqual(c.body);
      }
    });
  });
});
