// Centralized DaemonSnapshot factory for palette / tool tests.
//
// Why this lives here, not inline in each test file: ParamSelectPhase.test.tsx,
// CommandPalette.test.tsx, and lib/tools.test.ts each used to build their own
// `makeDaemonSnapshot` helper. Every wire-shape change (e.g. session-config-
// extension adding `pushCommands`, ADR-0044 adding `activeOccupant`) forced
// the same edit in N places. Funneling all snapshot construction through
// `mkSnapshot` collapses that fan-out into one file so future wire diffs
// touch a single line each.
//
// Notes on imports:
//   - `DaemonSnapshot` lives in `lib/tools.ts` (the read-only projection
//     handed to ToolDef.submit / disabledReason), NOT in `wire/server.ts`.
//     The wire layer ships `SessionInfo` and `HelloFrame`/`ViewUpdateFrame`;
//     the snapshot is assembled by `selectDaemonSnapshot` in store/daemon.
//   - "Project" on the snapshot is `SessionConfigProject` (path / isGit /
//     isSandboxed), defined in `api/sessions.ts` — there is no separate
//     `ProjectInfo`. We re-export that as `ProjectInfo` below for tests
//     that want the conventional name without learning the real source.
import type { SessionConfigProject } from "../../api/sessions";
import type { DaemonOccupant, DaemonSnapshot } from "../../lib/tools";
import type { SessionInfo } from "../../wire/server";

// Re-exported under the conventional names test files reach for. Keeping
// the alias here means tests can `import type { ProjectInfo } from
// "../test/fixtures/daemon"` without knowing the real type lives under
// api/sessions.
export type ProjectInfo = SessionConfigProject;
export type { DaemonOccupant, DaemonSnapshot, SessionInfo };

export interface MkSnapshotInput {
  /**
   * Partial project entries. Each is merged onto a default
   * `{ path: "/repo${i+1}", isGit: false, isSandboxed: false }` so callers
   * only spell out the fields they're asserting on. Default `undefined`
   * yields the FR-A4 "no projects" case (`projects: []`).
   */
  projects?: Partial<SessionConfigProject>[];
  /**
   * Partial session entries. Each is merged onto a default SessionInfo with
   * a stable `s${i+1}` id and `view.card.title === id` so list-rendering
   * tests have a non-empty card by default. Override anything that matters.
   */
  sessions?: Partial<SessionInfo>[];
  /** Active session id projection — default `null` (no selection). */
  activeSessionID?: string | null;
  /** Push command catalog — default `[]`. */
  pushCommands?: string[];
  /** Pane occupancy for push-scope gating (FR-006). Default undefined. */
  activeOccupant?: DaemonOccupant;
}

function mkProject(p: Partial<SessionConfigProject>, i: number): SessionConfigProject {
  return {
    path: p.path ?? `/repo${i + 1}`,
    isGit: p.isGit ?? false,
    isSandboxed: p.isSandboxed ?? false,
  };
}

function mkSessionInfo(s: Partial<SessionInfo>, i: number): SessionInfo {
  const id = s.id ?? `s${i + 1}`;
  // Defaults mirror lib/tools.test.ts:sessionFixture and store/daemon.test.ts:
  // mkSession so call sites that switch over to mkSnapshot don't observe a
  // surprise diff in card title / created_at.
  return {
    id,
    project: s.project ?? "/p",
    command: s.command ?? "claude",
    created_at: s.created_at ?? "2026-06-24T00:00:00Z",
    view: s.view ?? { card: { title: id } },
    ...s,
  };
}

/**
 * Build a DaemonSnapshot for tests. All inputs are optional; the default
 * call (`mkSnapshot()`) returns the canonical "empty daemon" snapshot
 * (`sessions: [], projects: [], pushCommands: [], activeSessionID: null`)
 * used by FR-A4 / scopeDisabledReason("standard") happy-paths.
 */
export function mkSnapshot(input: MkSnapshotInput = {}): DaemonSnapshot {
  const projects = (input.projects ?? []).map(mkProject);
  const sessions = (input.sessions ?? []).map(mkSessionInfo);
  const snap: DaemonSnapshot = {
    sessions,
    activeSessionID: input.activeSessionID ?? null,
    projects,
    pushCommands: input.pushCommands ?? [],
  };
  // activeOccupant is optional on DaemonSnapshot (omit when undefined so
  // `'activeOccupant' in snap` reflects caller intent — some tests assert
  // on the key's presence).
  if (input.activeOccupant !== undefined) {
    snap.activeOccupant = input.activeOccupant;
  }
  return snap;
}
