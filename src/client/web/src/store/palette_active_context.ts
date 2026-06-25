// Pure helpers for ActiveContextHeader display data.
// UAC-001 / UAC-013 / FR-009 / FR-025 / FR-027 / FR-028
//
// No store state or actions here — those live in the active-context-slice task.
// This module is a plain input → output transformer so it is trivially testable.

import type { SessionConfigProject } from "../api/sessions";
import type { SessionInfo } from "../wire/server";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/**
 * Discriminated union returned by deriveActiveContext.
 *
 * - 'resolved': an active session was found and its project path matched a
 *   known project. projBase / sid8 / fullPath / fullSessionId are all
 *   populated.
 * - 'unknown': an active session ID is present but either the session was
 *   not found in the sessions list, or the session's project path could not
 *   be matched in the projects list.
 * - 'none': no active session ID (null / empty string).
 */
export type ActiveContextSnapshot =
  | {
      kind: "resolved";
      projBase: string; // disambiguator-augmented basename (FR-027)
      sid8: string; // activeSessionID.slice(0, 8) (FR-028)
      fullPath: string; // projects[].path full value
      fullSessionId: string; // activeSessionID full value
    }
  | {
      kind: "unknown";
      // session ID is present but project resolution failed
      sid8: string;
      fullSessionId: string;
    }
  | {
      kind: "none"; // activeSessionID is null / empty
    };

// ---------------------------------------------------------------------------
// projBase (FR-027)
// ---------------------------------------------------------------------------

/**
 * Return the display basename for a project path, with an optional
 * disambiguator suffix ' (under <parent>)' when the same basename appears in
 * more than one project.
 *
 * Rules:
 *   - trim-empty input → return '' as-is
 *   - '/' or '\' terminated input → return input as-is (no basename extraction)
 *   - Both '/' and '\' count as path separators (Windows compat)
 *   - Disambiguator ' (under <parent>)' is only added when another project
 *     shares the same basename (case-sensitive)
 */
export function projBase(
  targetPath: string,
  allProjects: ReadonlyArray<SessionConfigProject>,
): string {
  const trimmed = targetPath.trim();
  if (trimmed === "") return targetPath;

  // If path ends with a separator, return as-is (no meaningful basename).
  if (trimmed.endsWith("/") || trimmed.endsWith("\\")) return targetPath;

  const base = extractBasename(trimmed);

  // Check for collision: does any other project share this basename?
  const hasCollision = allProjects.some((p) => {
    if (p.path === targetPath) return false; // same entry, skip
    const pTrimmed = p.path.trim();
    if (pTrimmed === "") return false;
    if (pTrimmed.endsWith("/") || pTrimmed.endsWith("\\")) return false;
    return extractBasename(pTrimmed) === base;
  });

  if (!hasCollision) return base;

  const parent = extractParentBasename(trimmed);
  return `${base} (under ${parent})`;
}

/** Extract the last path segment using '/' or '\' as separator. */
function extractBasename(p: string): string {
  const lastSep = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
  if (lastSep === -1) return p;
  return p.slice(lastSep + 1);
}

/**
 * Extract the parent directory's basename (one level up from the basename).
 * Returns '' when there is no parent segment.
 */
function extractParentBasename(p: string): string {
  const lastSep = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
  if (lastSep === -1) return "";
  const withoutBase = p.slice(0, lastSep);
  if (withoutBase === "") return "";
  return extractBasename(withoutBase);
}

// ---------------------------------------------------------------------------
// sid8 (FR-028)
// ---------------------------------------------------------------------------

/**
 * Return the first 8 characters of activeSessionID.
 * Uniqueness is not guaranteed — this is a display hint only.
 * The full session ID is returned separately (fullSessionId) for tooltips.
 */
export function sid8(activeSessionID: string): string {
  return activeSessionID.slice(0, 8);
}

// ---------------------------------------------------------------------------
// deriveActiveContext (ADR-0058, FR-009, FR-025)
// ---------------------------------------------------------------------------

/**
 * Derive the active context snapshot from the current daemon state.
 *
 * ADR-0058 fallback: SessionInfo.project is matched against projects[].path.
 * When SessionInfo.projectPath is added to the wire in a future PR, this
 * function should prefer that field over session.project.
 */
export function deriveActiveContext(
  activeSessionID: string | null,
  sessions: ReadonlyArray<SessionInfo>,
  projects: ReadonlyArray<SessionConfigProject>,
): ActiveContextSnapshot {
  if (activeSessionID === null || activeSessionID === undefined || activeSessionID === "") {
    return { kind: "none" };
  }

  const session = sessions.find((s) => s.id === activeSessionID);

  // ADR-0058: use session.project as the project key (path).
  const projectKey = session?.project;
  if (!projectKey) {
    return {
      kind: "unknown",
      sid8: sid8(activeSessionID),
      fullSessionId: activeSessionID,
    };
  }

  const proj = projects.find((p) => p.path === projectKey);
  if (!proj) {
    return {
      kind: "unknown",
      sid8: sid8(activeSessionID),
      fullSessionId: activeSessionID,
    };
  }

  return {
    kind: "resolved",
    projBase: projBase(proj.path, projects),
    sid8: sid8(activeSessionID),
    fullPath: proj.path,
    fullSessionId: activeSessionID,
  };
}

// ---------------------------------------------------------------------------
// StateCreator slice (ADR-0056)
// ---------------------------------------------------------------------------

import type { StateCreator } from "zustand";

export interface ActiveContextSliceState {
  activeContextSnapshot: ActiveContextSnapshot;
  flashSeq: number;
  announceSeq: number;
}

export interface ActiveContextSliceActions {
  setActiveContextSnapshot(next: ActiveContextSnapshot): void;
}

export type ActiveContextSlice = ActiveContextSliceState & ActiveContextSliceActions;

// initialActiveContextState is exported so palette.ts can merge it into
// close() / back() resets (snapshot back to { kind: 'none' }).
// flashSeq / announceSeq are intentionally NOT reset on close — they are
// monotonic counters and must remain so across palette lifecycle events.
export const initialActiveContextState: ActiveContextSliceState = {
  activeContextSnapshot: { kind: "none" },
  flashSeq: 0,
  announceSeq: 0,
};

// createActiveContextSlice is the zustand StateCreator for the active-context
// slice. setActiveContextSnapshot compares fullSessionId (or null for 'none')
// between prev and next:
//   - same id → update snapshot only (disambiguator-only change is silent)
//   - different id → update snapshot + bump flashSeq + announceSeq
//   - none -> none → same id (null === null) → seq unchanged
export const createActiveContextSlice: StateCreator<
  ActiveContextSlice,
  [],
  [],
  ActiveContextSlice
> = (set, get) => ({
  ...initialActiveContextState,
  setActiveContextSnapshot(next) {
    const prev = get().activeContextSnapshot;
    const prevId = prev.kind === "none" ? null : prev.fullSessionId;
    const nextId = next.kind === "none" ? null : next.fullSessionId;
    if (prevId === nextId) {
      set({ activeContextSnapshot: next });
      return;
    }
    set((s) => ({
      activeContextSnapshot: next,
      flashSeq: s.flashSeq + 1,
      announceSeq: s.announceSeq + 1,
    }));
  },
});
