// Reuse the base unit setup (jest-dom matchers + deterministic web storage
// polyfill + per-test storage reset).
import "../setup";

import { afterAll, afterEach, beforeAll } from "vitest";
import { server } from "./server";

// Fail on any request the active test did not explicitly mock, so a missing
// handler surfaces as a test error rather than a real network call.
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
