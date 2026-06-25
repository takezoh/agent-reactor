import { create } from "zustand";
import type { SessionConfig, SessionConfigProject } from "../api/sessions";
import type { DaemonOccupant, DaemonSnapshot } from "../lib/tools";
import type { HelloFrame, SessionInfo, ViewUpdateFrame } from "../wire/server";

export type ConnectionStatus = "connecting" | "open" | "reconnecting" | "closed";

// SessionConfigSlice mirrors the subset of GET /api/session-config the
// palette needs to gate behavior: the projects list (with
// isGit/isSandboxed flags driving the new-session worktree/host toggles per
// FR-013/FR-014) and the push_commands enumeration (FR-027, fed to the
// dynamic push scope by tools-registry-dynamic-push). Kept as its own slice
// (rather than packed into the existing socket frames) because /api/session-
// config is REST-only by ADR-0041 / ADR-0030 — it never rides the WS view
// update path. null means "not fetched yet"; consumers should fall back to
// empty arrays in that case.
export type SessionConfigSlice = {
  projects: SessionConfigProject[];
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

  // actions
  seedHello: (frame: HelloFrame) => void;
  applyViewUpdate: (frame: ViewUpdateFrame) => void;
  selectSession: (id: string | null) => void;
  setStatus: (status: ConnectionStatus) => void;
  setDaemonDisconnected: (v: boolean) => void;
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
};

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
    pushCommands: state.sessionConfig?.pushCommands ?? [],
  };
}

export const useDaemonStore = create<DaemonState>()((set) => ({
  ...initialState,
  seedHello: (frame) =>
    set((s) => ({
      sessions: frame.sessions,
      activeSessionID: frame.activeSessionID,
      features: frame.features,
      serverTime: frame.serverTime,
      // activeOccupant: HelloFrame.activeOccupant is optional. When the
      // server emits it (post 2026-06-24), seed the daemon-global occupant
      // so the palette's push scope (FR-005/FR-006) can gate correctly on
      // the very first connection. When absent (legacy server) we leave
      // the existing value alone — `undefined` already maps to "no frame"
      // via the fail-closed path in scopeDisabledReason, so reading a
      // stale value is safer than overwriting with undefined here.
      activeOccupant: frame.activeOccupant ?? s.activeOccupant,
    })),
  applyViewUpdate: (frame) =>
    set((s) => {
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
        activeSessionID:
          frame.activeSessionID === undefined ? s.activeSessionID : frame.activeSessionID,
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
      };
    }),
  selectSession: (id) => set({ activeSessionID: id }),
  setStatus: (status) => set({ status }),
  setDaemonDisconnected: (v) => set({ daemonDisconnected: v }),
  setSessionConfig: (cfg) =>
    set({
      sessionConfig: {
        projects: cfg.projects,
        pushCommands: cfg.pushCommands,
      },
    }),
  reset: () => set(initialState),
}));
