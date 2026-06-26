import { create } from "zustand";
import type { SessionConfig, SessionConfigProject } from "../api/sessions";
import type { DaemonOccupant, DaemonSnapshot } from "../lib/tools";
import type { HelloFrame, SessionInfo, ViewUpdateFrame } from "../wire/server";

// Mirror of Go's config.DefaultWorkspaceName — the daemon emits this literal
// when a project has no named workspace in its settings.toml.
export const DEFAULT_WORKSPACE = "default";

/** workspaceOf — TUI parity (workspace.go). Empty string → DEFAULT_WORKSPACE. */
export function workspaceOf(s: SessionInfo): string {
  const ws = s.workspace?.trim();
  return ws ? ws : DEFAULT_WORKSPACE;
}

export type ConnectionStatus = "connecting" | "open" | "reconnecting" | "closed";

// SessionConfigSlice mirrors the subset of GET /api/session-config the
// palette needs to gate behavior: the projects list (with
// isGit/isSandboxed flags driving the new-session worktree/host toggles per
// FR-013/FR-014), the curated [session].commands list (the same picker the
// TUI palette uses — feeds the new-session "Command" dynamic listbox) and
// the push_commands enumeration (FR-027, fed to the dynamic push scope by
// tools-registry-dynamic-push). Kept as its own slice (rather than packed
// into the existing socket frames) because /api/session-config is REST-only
// by ADR-0041 / ADR-0030 — it never rides the WS view update path. null
// means "not fetched yet"; consumers should fall back to empty arrays.
export type SessionConfigSlice = {
  projects: SessionConfigProject[];
  commands: string[];
  pushCommands: string[];
};

export type DaemonState = {
  sessions: SessionInfo[];
  activeSessionID: string | null;
  // activeOccupant mirrors the daemon-global ActiveOccupant ('main' | 'log' |
  // 'frame' — what currently occupies pane 0.1 in the TUI). The Web UI uses
  // it only to gate the palette's push scope (FR-005, FR-006): push is
  // available iff there is an active session AND its pane 0.1 holds a frame
  // driver. Optional because the wire does not yet carry this signal; until
  // it does, the field stays undefined and scopeDisabledReason treats it as
  // "no frame" → push is fail-closed disabled. Once the wire grows an
  // ActiveOccupant attribute on HelloFrame/ViewUpdateFrame, seedHello /
  // applyViewUpdate are the right hooks to populate it from.
  activeOccupant?: DaemonOccupant;
  features: string[];
  serverTime: number;
  status: ConnectionStatus;
  // control frame で daemon-disconnected が来たかどうか。StatusBanner が参照。
  daemonDisconnected: boolean;
  // sessionConfig is the cached REST-fetched slice of GET /api/session-config
  // (ADR-0041). null until the first fetch lands; callers should treat null
  // as "empty projects, empty pushCommands" rather than blocking the UI.
  sessionConfig: SessionConfigSlice | null;

  // selectedWorkspace mirrors TUI Model.selectedWorkspace (workspace.go).
  // The session list is partitioned by workspace; this picks which
  // partition is visible. Defaults to DEFAULT_WORKSPACE. When the user
  // selects a session belonging to a different workspace, the store
  // automatically follows (TUI sidebar_events.go parity).
  selectedWorkspace: string;

  // foldedProjects collects project names whose children are currently
  // collapsed in the sidebar. Persists across view-updates so the user's
  // fold state isn't reset by every daemon push. TUI parity:
  // Model.folded (sidebar_items.go:74).
  foldedProjects: ReadonlySet<string>;

  // actions
  seedHello: (frame: HelloFrame) => void;
  applyViewUpdate: (frame: ViewUpdateFrame) => void;
  selectSession: (id: string | null) => void;
  setStatus: (status: ConnectionStatus) => void;
  setDaemonDisconnected: (v: boolean) => void;
  setSelectedWorkspace: (workspace: string) => void;
  toggleProjectFold: (project: string) => void;
  // setSessionConfig replaces the cached REST snapshot. Callers pass the
  // result of makeSessionsApi().getSessionConfig() (already normalized into
  // {projects: SessionConfigProject[], pushCommands: string[]} by the api-
  // client layer). The action stores only the two fields the palette / scope
  // gating consume — commands / default_command / project_roots remain owned
  // by the palette's lazy session-config fetch path (ParamSelectPhase /
  // new-session ToolDef, post f2 CreateSessionForm removal — ADR-0043).
  setSessionConfig: (cfg: SessionConfig) => void;
  reset: () => void;
};

const initialState = {
  sessions: [] as SessionInfo[],
  activeSessionID: null as string | null,
  activeOccupant: undefined as DaemonOccupant | undefined,
  features: [] as string[],
  serverTime: 0,
  status: "connecting" as ConnectionStatus,
  daemonDisconnected: false,
  sessionConfig: null as SessionConfigSlice | null,
  selectedWorkspace: DEFAULT_WORKSPACE,
  foldedProjects: new Set<string>() as ReadonlySet<string>,
};

/** Workspaces currently represented by at least one session, sorted with
 *  DEFAULT_WORKSPACE first, then alphabetical (TUI collectWorkspaces parity).
 *  DEFAULT_WORKSPACE is always included even when no session belongs to it,
 *  so the user always has a "home" partition to return to after deleting
 *  the last session in a named workspace. */
export function selectDistinctWorkspaces(sessions: readonly SessionInfo[]): string[] {
  const set = new Set<string>([DEFAULT_WORKSPACE]);
  for (const s of sessions) {
    set.add(workspaceOf(s));
  }
  const arr = Array.from(set);
  arr.sort((a, b) => {
    if (a === DEFAULT_WORKSPACE) return -1;
    if (b === DEFAULT_WORKSPACE) return 1;
    return a.localeCompare(b);
  });
  return arr;
}

/** Project basename — last non-empty path component of session.project. */
export function projectBasename(project: string): string {
  const trimmed = project.replace(/\/+$/, "");
  const i = trimmed.lastIndexOf("/");
  return i < 0 ? trimmed : trimmed.slice(i + 1);
}

export type ProjectGroup = {
  /** Display name (basename of project path). */
  project: string;
  /** Full project path. Distinct paths sharing a basename are kept apart
   *  via this field, so /a/repo and /b/repo become two groups (with the
   *  same display name) rather than collapsing into one. */
  projectPath: string;
  sessions: SessionInfo[];
};

/** Sessions belonging to the given workspace, grouped by project path
 *  (NOT just basename — distinct paths sharing a basename stay separate;
 *  TUI sidebar_items.go:50-62 uses byProject keyed on s.Name() which is
 *  also basename, but the wire's `project` field is the source of truth
 *  for identity here). Groups are sorted alphabetically by display name. */
export function groupSessionsByProject(
  sessions: readonly SessionInfo[],
  workspace: string,
): ProjectGroup[] {
  const groups = new Map<string, ProjectGroup>();
  for (const s of sessions) {
    if (workspaceOf(s) !== workspace) continue;
    const projectPath = s.project ?? "";
    const basename = projectBasename(projectPath);
    if (!basename) continue;
    const existing = groups.get(projectPath);
    if (existing) {
      existing.sessions.push(s);
    } else {
      groups.set(projectPath, {
        project: basename,
        projectPath: projectPath || basename,
        sessions: [s],
      });
    }
  }
  return Array.from(groups.values()).sort((a, b) => {
    const c = a.project.localeCompare(b.project);
    return c !== 0 ? c : a.projectPath.localeCompare(b.projectPath);
  });
}

// DaemonSnapshotSource is the structural subset of DaemonState that
// selectDaemonSnapshot consumes. Defining it as a structural type (not as
// `Pick<DaemonState, ...>`) lets React-side callers pass narrowed inputs
// without having to satisfy the full DaemonState (which would force them
// to fake the action functions just to call the selector). Adding a new
// field here is a contract change visible at every call site — keep it
// minimal.
export type DaemonSnapshotSource = {
  sessions: DaemonState["sessions"];
  activeSessionID: DaemonState["activeSessionID"];
  // activeOccupant is optional to mirror DaemonState (the field itself is
  // optional on the store — wire-absent reads as undefined, fail-closed
  // for push). Declaring it required here would force every test fixture
  // to invent a value just to call selectDaemonSnapshot.
  activeOccupant?: DaemonState["activeOccupant"];
  sessionConfig: DaemonState["sessionConfig"];
};

// selectDaemonSnapshot is the read-only projection consumers (CommandPalette,
// ToolSelectPhase, ParamSelectPhase) feed to
// scopeDisabledReason / ToolDef.disabledReason / listTools (ADR-0047). It
// assembles the DaemonSnapshot shape from the live daemon store without
// forcing each caller to repeat the field plumbing. projects /
// pushCommands come from the REST-fetched sessionConfig slice (ADR-0041);
// when the fetch has not yet landed we expose empty arrays so the
// standard-scope path stays usable and the push scope fail-closed-disables.
//
// Accepts DaemonSnapshotSource (a structural subset) so React consumers
// can call this with subscribed primitives directly — keeps useMemo deps
// honest (Biome's useExhaustiveDependencies sees the values consumed) and
// avoids the indirection of `useDaemonStore.getState()` inside useMemo.
export function selectDaemonSnapshot(state: DaemonSnapshotSource): DaemonSnapshot {
  return {
    sessions: state.sessions,
    activeSessionID: state.activeSessionID,
    activeOccupant: state.activeOccupant,
    projects: state.sessionConfig?.projects ?? [],
    commands: state.sessionConfig?.commands ?? [],
    pushCommands: state.sessionConfig?.pushCommands ?? [],
  };
}

export const useDaemonStore = create<DaemonState>()((set) => ({
  ...initialState,
  seedHello: (frame) =>
    set((s) => {
      // Follow the daemon's active session into its workspace on hello so
      // the partition matching activeSessionID is what the user sees first
      // (TUI sidebar_events.go parity — `m.selectedWorkspace = workspaceOf(s)`).
      let ws = s.selectedWorkspace;
      if (frame.activeSessionID !== null) {
        const active = frame.sessions.find((x) => x.id === frame.activeSessionID);
        if (active) ws = workspaceOf(active);
      }
      // Reset to DEFAULT_WORKSPACE if the previously selected workspace no
      // longer exists in the seeded session set (TUI rebuildItems:38-47).
      const known = selectDistinctWorkspaces(frame.sessions);
      if (!known.includes(ws)) ws = DEFAULT_WORKSPACE;
      return {
        sessions: frame.sessions,
        activeSessionID: frame.activeSessionID,
        features: frame.features,
        serverTime: frame.serverTime,
        activeOccupant: frame.activeOccupant ?? s.activeOccupant,
        selectedWorkspace: ws,
      };
    }),
  applyViewUpdate: (frame) =>
    set((s) => {
      // Resolve effective activeSessionID.
      const nextActiveId =
        frame.activeSessionID === undefined ? s.activeSessionID : frame.activeSessionID;
      // selectedWorkspace policy: this is a USER preference (set by chip click
      // / selectSession). On view-update we leave it alone unless:
      //   (a) the workspace no longer exists in the pushed session set → reset
      //       to DEFAULT_WORKSPACE so the UI never shows an empty unreachable
      //       partition (TUI rebuildItems:38-47 parity);
      //   (b) the active session actually CHANGED via the wire (another tab
      //       switched it) AND that change crosses workspaces → follow, so the
      //       partition matching the new active is visible (TUI parity).
      // The pre-fix code re-applied (b) on every push regardless of whether
      // activeId changed, silently undoing chip clicks. (code-review #1.)
      let ws = s.selectedWorkspace;
      const known = selectDistinctWorkspaces(frame.sessions);
      if (!known.includes(ws)) ws = DEFAULT_WORKSPACE;
      if (nextActiveId !== null && nextActiveId !== s.activeSessionID) {
        const active = frame.sessions.find((x) => x.id === nextActiveId);
        if (active) ws = workspaceOf(active);
      }
      // best-effort identity preservation: keep the previous SessionInfo
      // object when its JSON shape is structurally unchanged. Cheap deep
      // compare via JSON.stringify is fine here (sessions[] is small —
      // 10s of entries — and the cost runs once per daemon push, ADR 0023).
      const byId = new Map(s.sessions.map((x) => [x.id, x]));
      const next = frame.sessions.map((incoming) => {
        const prev = byId.get(incoming.id);
        if (prev && JSON.stringify(prev) === JSON.stringify(incoming)) {
          return prev;
        }
        return incoming;
      });
      return {
        sessions: next,
        activeSessionID: nextActiveId,
        // ViewUpdateFrame.activeOccupant is optional. When the frame
        // carries it (post 2026-06-24 server emit), we overwrite the
        // current value live so a frame pushed / popped by another driver
        // client toggles the palette's push availability without a
        // reconnect. When absent we leave the current value alone (the
        // legacy-server / partial-update path); explicit "no occupant"
        // does not exist on the wire — the daemon would simply omit the
        // field.
        activeOccupant:
          frame.activeOccupant === undefined ? s.activeOccupant : frame.activeOccupant,
        selectedWorkspace: ws,
      };
    }),
  selectSession: (id) =>
    set((s) => {
      // TUI parity (sidebar_events.go:49-58): when the user picks a session
      // belonging to a different workspace, follow it. The switcher then
      // shows the partition the picked session lives in.
      let ws = s.selectedWorkspace;
      if (id !== null) {
        const sess = s.sessions.find((x) => x.id === id);
        if (sess) ws = workspaceOf(sess);
      }
      return { activeSessionID: id, selectedWorkspace: ws };
    }),
  setStatus: (status) => set({ status }),
  setDaemonDisconnected: (v) => set({ daemonDisconnected: v }),
  setSessionConfig: (cfg) =>
    set({
      sessionConfig: {
        projects: cfg.projects,
        commands: cfg.commands,
        pushCommands: cfg.pushCommands,
      },
    }),
  setSelectedWorkspace: (workspace) => set({ selectedWorkspace: workspace }),
  toggleProjectFold: (project) =>
    set((s) => {
      const next = new Set(s.foldedProjects);
      if (next.has(project)) next.delete(project);
      else next.add(project);
      return { foldedProjects: next };
    }),
  reset: () =>
    set(() => ({
      ...initialState,
      // Fresh Set so a reset never aliases the module-level instance —
      // toggleProjectFold always replaces the Set, but a future caller that
      // mutates in place would otherwise leak across reset boundaries.
      foldedProjects: new Set<string>(),
    })),
}));
