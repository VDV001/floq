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
  });
});
