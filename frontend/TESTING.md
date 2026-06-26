# Frontend testing conventions

This document codifies how the frontend test suite mocks the network. It exists
because the suite once shipped two contradictory conventions and a real bug
(P0-3 HITL: approve/reject endpoints returned `204` and every operator click
surfaced "не удалось одобрить" on success) slipped past nine component tests
that mocked the `api` object but not the underlying `fetch` contract.

## Two mock styles

| | Style 1 — fetch global | Style 2 — api object |
|---|---|---|
| Mechanism | `vi.stubGlobal("fetch", fetchMock)` | `vi.mock("@/lib/api", ...)` |
| What runs | Real `apiFetch` round-trips through every request (URL, headers, status-code handling) | `apiFetch` is replaced wholesale; tests assert on call args of the api method |
| Catches | Wire-format bugs: status codes (e.g. 204), auth refresh, body shape, Content-Type | UI behaviour: rendering, state transitions, button states |
| Misses | Tightly couples to `apiFetch` internals; verbose | Anything below the api object (a bad URL, a 204 that needs special handling, missing Authorization header) |
| Example | [`src/lib/api.test.ts`](src/lib/api.test.ts) | [`src/components/leads/PendingReplySection.test.tsx`](src/components/leads/PendingReplySection.test.tsx) |

## Rules

1. **Every endpoint added to `src/lib/api.ts` MUST have a style-1 contract test**
   in `src/lib/api.test.ts`. The contract test asserts URL, method, request
   body shape, and any non-trivial response handling (notably: empty-body
   responses such as `204 No Content`, which trip `res.json()`). See
   `api.test.ts` "API method contracts" describe block for the pattern.

2. **Component / hook tests SHOULD use style 2** (`vi.mock("@/lib/api", ...)`).
   The thing under test is UI behaviour, not wire format. Mocking the api
   object keeps these tests fast and stable.

3. **A style-2 test does not exempt the endpoint from rule 1.** If you mock
   `api.fooBar` in a component test, `api.test.ts` must already exercise
   `fooBar` end-to-end. New endpoint → contract test first, then component
   test.

4. **A style-1 contract test belongs to every endpoint that returns a status
   code other than `200`** — including but not limited to `201`, `204`, `401`
   (refresh), redirects, and any 4xx/5xx handling specific to that endpoint.
   "Returns nothing on success" is a contract.

## Mock hygiene

- **Use `vi.resetAllMocks()` in `beforeEach`, not `vi.clearAllMocks()`** when
  test scenarios vary call-by-call. `clearAllMocks` resets call history but
  *preserves* implementations including the `mockResolvedValueOnce` queue,
  which leaks between tests and produces hard-to-trace order-dependent
  failures.
- Style 1 tests should call `vi.resetModules()` before re-importing the api
  module so module-level state (constants, the `API_BASE` snapshot) is fresh.
  See `api.test.ts:42`.
- Style 1 tests should call `vi.unstubAllGlobals()` in `afterEach` to release
  `fetch`, `localStorage`, `URL` and `location` stubs. See `api.test.ts:47-49`.

## Authoring a new endpoint — minimum bar

1. Add the method to `src/lib/api.ts`.
2. Add the contract test to `src/lib/api.test.ts` under the appropriate
   describe block:
   - "apiFetch" — for any new wire-format behaviour (status code handling,
     headers, auth flow).
   - "API method contracts" — for the URL + method + body assertion.
3. Add the component test, mocking via style 2.

## Why this is enforced via PR checklist (and not eslint)

A static linter cannot prove that a contract test for `api.fooBar` actually
exercises the wire format — only that some assertion mentions `fooBar`. The
acceptance criterion in PR review is human: does `src/lib/api.test.ts` round
through `apiFetch` for this endpoint? See the PR template's "Frontend mocks"
checkbox.

## References

- [`src/lib/api.ts:57-61`](src/lib/api.ts) — the 204 special-case that the
  contract test pins.
- [`src/lib/api.test.ts:102-122`](src/lib/api.test.ts) — the contract test
  added in the P0-3 fix-up that catches a regression in 204 handling.
- Issue #54 — the convention's origin.
