import { setupServer } from "msw/node";

// The integration suite drives real page + hook + lib/api.ts code and only
// mocks the network boundary. Each flow registers its own handlers via
// server.use(); the base server starts with none so any unmocked request
// fails loudly instead of silently hitting a real backend.
export const server = setupServer();

// API_BASE in lib/api.ts (NEXT_PUBLIC_API_URL || http://localhost:8080).
export const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export const url = (path: string): string => `${API_BASE}${path}`;
