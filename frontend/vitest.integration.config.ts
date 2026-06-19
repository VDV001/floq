import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

// Integration layer: renders real pages/hooks driving the real lib/api.ts
// client, with only the network boundary mocked via MSW. Distinct from the
// unit config, which mocks @/lib/api directly.
export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/__tests__/integration/setup.ts"],
    include: ["src/**/*.int.test.{ts,tsx}"],
    // Integration flows drive real user interactions through MSW; they are
    // legitimately slower than unit tests and flake under worker contention
    // (a pending fetch can blow the default 5s timeout). Run them in a single
    // worker with a generous timeout for stability.
    testTimeout: 15000,
    fileParallelism: false,
    coverage: {
      provider: "v8",
      // Report coverage only over the code these flows actually exercise
      // end-to-end (pages, hooks, the api client), so the ≥60% gate reflects
      // integration reach rather than the whole component library.
      include: [
        "src/app/**/*.{ts,tsx}",
        "src/hooks/**/*.{ts,tsx}",
        "src/lib/api.ts",
      ],
      exclude: ["src/**/*.test.*", "src/**/*.int.test.*", "src/__tests__/**"],
      reportOnFailure: true,
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
