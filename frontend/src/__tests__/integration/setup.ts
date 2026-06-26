// Reuse the base unit setup (jest-dom matchers + deterministic web storage
// polyfill + per-test storage reset).
import "../setup";

import { afterAll, afterEach, beforeAll } from "vitest";
import { server } from "./server";

// The base setup imported above also registers an afterEach (web-storage
// reset). Vitest runs afterEach hooks in registration order, so storage is
// cleared before this file's resetHandlers — both run every test, no leak.

// Fail on any request the active test did not explicitly mock, so a missing
// handler surfaces as a test error rather than a real network call.
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
