// sessionsApiGuard.ts — guard function to validate SessionsApi shape.
//
// Extracted from CommandPalette.tsx so both CommandPalette and ToolSelectPhase
// can import the same validation logic without duplication.
//
// FR refs: FR-025

import type { ToolCtx } from "./tools";

// SessionsApi method names required at ToolCtx construction time. If the
// httpFactory returns something missing any of these, submit() would crash
// deep inside with a confusing 'undefined is not a function'. We check up
// front so the root cause (broken factory) is on the same stack as the failure.
export const REQUIRED_HTTP_METHODS = ["createSession", "pushCommand", "getSessionConfig"] as const;

export function isValidSessionsApi(http: unknown): http is ToolCtx["http"] {
  if (http === null || typeof http !== "object") return false;
  const obj = http as Record<string, unknown>;
  return REQUIRED_HTTP_METHODS.every((name) => typeof obj[name] === "function");
}
