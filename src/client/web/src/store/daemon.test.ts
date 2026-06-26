import { beforeEach, describe, expect, it } from "vitest";
import type { SessionConfig } from "../api/sessions";
import type { HelloFrame, SessionInfo, ViewUpdateFrame } from "../wire/server";
import {
  DEFAULT_WORKSPACE,
  groupSessionsByProject,
  projectBasename,
  selectDaemonSnapshot,
  selectDistinctWorkspaces,
  useDaemonStore,
  workspaceOf,
} from "./daemon";

function mkSession(id: string, overrides: Partial<SessionInfo> = {}): SessionInfo {
  return {
    id,
    project: "p",
    command: "claude",
    created_at: "2026-06-20T00:00:00Z",
    view: { card: { title: id }, status: "running" },
    ...overrides,
  };
}

describe("daemonStore", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
  });

  it("seedHello populates sessions/features/serverTime/activeSessionID", () => {
    const frame: HelloFrame = {
      k: "h",
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "t" }, status: "running" },
        },
      ],
      activeSessionID: "s1",
      features: ["surface"],
      serverTime: 12345,
    };
    useDaemonStore.getState().seedHello(frame);
    const s = useDaemonStore.getState();
    expect(s.sessions).toHaveLength(1);
    expect(s.activeSessionID).toBe("s1");
    expect(s.features).toEqual(["surface"]);
    expect(s.serverTime).toBe(12345);
  });

  it("applyViewUpdate replaces sessions and preserves activeSessionID when omitted", () => {
    useDaemonStore.setState({ activeSessionID: "preserved" });
    const frame: ViewUpdateFrame = {
      k: "v",
      sessions: [
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "t2" }, status: "stopped" },
        },
      ],
    };
    useDaemonStore.getState().applyViewUpdate(frame);
    expect(useDaemonStore.getState().activeSessionID).toBe("preserved");
    expect(useDaemonStore.getState().sessions[0]?.id).toBe("s2");
  });

  it("applyViewUpdate overrides activeSessionID when provided", () => {
    useDaemonStore.setState({ activeSessionID: "old" });
    useDaemonStore.getState().applyViewUpdate({
      k: "v",
      sessions: [],
      activeSessionID: "new",
    });
    expect(useDaemonStore.getState().activeSessionID).toBe("new");
  });

  it("applyViewUpdate clears activeSessionID when explicit null is provided", () => {
    useDaemonStore.setState({ activeSessionID: "old" });
    useDaemonStore.getState().applyViewUpdate({
      k: "v",
      sessions: [],
      activeSessionID: null,
    });
    expect(useDaemonStore.getState().activeSessionID).toBeNull();
  });

  it("selectSession updates activeSessionID", () => {
    useDaemonStore.getState().selectSession("x");
    expect(useDaemonStore.getState().activeSessionID).toBe("x");
  });

  it("setStatus updates connection status", () => {
    useDaemonStore.getState().setStatus("reconnecting");
    expect(useDaemonStore.getState().status).toBe("reconnecting");
  });

  it("setDaemonDisconnected toggles flag", () => {
    useDaemonStore.getState().setDaemonDisconnected(true);
    expect(useDaemonStore.getState().daemonDisconnected).toBe(true);
  });

  it("applyViewUpdate replaces sessions with view payload", () => {
    useDaemonStore.setState({
      sessions: [mkSession("s1", { view: { card: { title: "old" }, status: "idle" } })],
    });
    useDaemonStore.getState().applyViewUpdate({
      k: "v",
      sessions: [mkSession("s1", { view: { card: { title: "new" }, status: "running" } })],
    });
    expect(useDaemonStore.getState().sessions[0]?.view.card.title).toBe("new");
  });

  it("applyViewUpdate preserves identity of unchanged sessions", () => {
    const session = mkSession("s1");
    useDaemonStore.setState({ sessions: [session] });
    useDaemonStore.getState().applyViewUpdate({
      k: "v",
      sessions: [mkSession("s1")],
    });
    expect(useDaemonStore.getState().sessions[0]).toBe(session);
  });

  it("applyViewUpdate produces new ref for changed session", () => {
    const session = mkSession("s1", { view: { card: { title: "s1" }, status: "idle" } });
    useDaemonStore.setState({ sessions: [session] });
    useDaemonStore.getState().applyViewUpdate({
      k: "v",
      sessions: [mkSession("s1", { view: { card: { title: "s1" }, status: "running" } })],
    });
    expect(useDaemonStore.getState().sessions[0]).not.toBe(session);
  });

  // --- sessionConfig (REST slice, ADR-0041) ---

  it("sessionConfig starts as null and snapshot exposes empty arrays", () => {
    expect(useDaemonStore.getState().sessionConfig).toBeNull();
    const snap = selectDaemonSnapshot(useDaemonStore.getState());
    expect(snap.projects).toEqual([]);
    expect(snap.pushCommands).toEqual([]);
  });

  it("setSessionConfig stores projects + pushCommands and snapshot reflects them", () => {
    const cfg: SessionConfig = {
      projectRoots: ["/home/me/code"],
      projectPaths: ["/home/me/repo-a"],
      projects: [
        { path: "/home/me/repo-a", isGit: true, isSandboxed: false },
        { path: "/home/me/repo-b", isGit: false, isSandboxed: true },
      ],
      commands: ["claude", "shell"],
      pushCommands: ["/clear", "/compact"],
    };
    useDaemonStore.getState().setSessionConfig(cfg);
    const s = useDaemonStore.getState();
    expect(s.sessionConfig).not.toBeNull();
    expect(s.sessionConfig?.projects).toHaveLength(2);
    expect(s.sessionConfig?.projects[0]).toEqual({
      path: "/home/me/repo-a",
      isGit: true,
      isSandboxed: false,
    });
    expect(s.sessionConfig?.pushCommands).toEqual(["/clear", "/compact"]);

    const snap = selectDaemonSnapshot(s);
    expect(snap.projects).toBe(s.sessionConfig?.projects);
    expect(snap.pushCommands).toBe(s.sessionConfig?.pushCommands);
  });

  it("setSessionConfig replaces the previous snapshot rather than merging", () => {
    useDaemonStore.getState().setSessionConfig({
      projectRoots: [],
      projectPaths: [],
      projects: [{ path: "/old", isGit: false, isSandboxed: false }],
      commands: [],
      pushCommands: ["/old"],
    });
    useDaemonStore.getState().setSessionConfig({
      projectRoots: [],
      projectPaths: [],
      projects: [{ path: "/new", isGit: true, isSandboxed: true }],
      commands: [],
      pushCommands: ["/new"],
    });
    const s = useDaemonStore.getState();
    expect(s.sessionConfig?.projects).toEqual([{ path: "/new", isGit: true, isSandboxed: true }]);
    expect(s.sessionConfig?.pushCommands).toEqual(["/new"]);
  });

  // --- activeOccupant (wire) ---

  it("seedHello with activeOccupant='frame' populates store activeOccupant", () => {
    const frame: HelloFrame = {
      k: "h",
      sessions: [],
      activeSessionID: null,
      activeOccupant: "frame",
      features: [],
      serverTime: 0,
    };
    useDaemonStore.getState().seedHello(frame);
    expect(useDaemonStore.getState().activeOccupant).toBe("frame");
  });

  it("seedHello without activeOccupant preserves the existing value (legacy server compat)", () => {
    useDaemonStore.setState({ activeOccupant: "frame" });
    const frame: HelloFrame = {
      k: "h",
      sessions: [],
      activeSessionID: null,
      features: [],
      serverTime: 0,
    };
    useDaemonStore.getState().seedHello(frame);
    // No activeOccupant on the wire → don't overwrite to undefined; the
    // existing fail-closed semantics ("no frame" → push disabled) handles
    // either reading.
    expect(useDaemonStore.getState().activeOccupant).toBe("frame");
  });

  it("applyViewUpdate with activeOccupant='main' overrides store activeOccupant", () => {
    useDaemonStore.setState({ activeOccupant: "frame" });
    useDaemonStore.getState().applyViewUpdate({
      k: "v",
      sessions: [],
      activeOccupant: "main",
    });
    expect(useDaemonStore.getState().activeOccupant).toBe("main");
  });

  it("applyViewUpdate without activeOccupant preserves existing value", () => {
    useDaemonStore.setState({ activeOccupant: "frame" });
    useDaemonStore.getState().applyViewUpdate({
      k: "v",
      sessions: [],
    });
    expect(useDaemonStore.getState().activeOccupant).toBe("frame");
  });

  it("selectDaemonSnapshot reflects activeOccupant from store", () => {
    useDaemonStore.setState({
      activeSessionID: "s1",
      activeOccupant: "frame",
    });
    const snap = selectDaemonSnapshot(useDaemonStore.getState());
    expect(snap.activeOccupant).toBe("frame");
    expect(snap.activeSessionID).toBe("s1");
  });

  it("reset clears sessionConfig back to null", () => {
    useDaemonStore.getState().setSessionConfig({
      projectRoots: [],
      projectPaths: [],
      projects: [{ path: "/a", isGit: false, isSandboxed: false }],
      commands: [],
      pushCommands: ["/p"],
    });
    useDaemonStore.getState().reset();
    expect(useDaemonStore.getState().sessionConfig).toBeNull();
  });

  // ─── Workspace + fold state (TUI parity) ───────────────────────────────

  it("DEFAULT_WORKSPACE matches the Go config.DefaultWorkspaceName sentinel", () => {
    expect(DEFAULT_WORKSPACE).toBe("default");
  });

  it("workspaceOf falls back to DEFAULT_WORKSPACE on empty/undefined", () => {
    expect(workspaceOf(mkSession("s1"))).toBe(DEFAULT_WORKSPACE);
    expect(workspaceOf(mkSession("s2", { workspace: "" }))).toBe(DEFAULT_WORKSPACE);
    expect(workspaceOf(mkSession("s3", { workspace: "prod" }))).toBe("prod");
  });

  it("initial selectedWorkspace is DEFAULT_WORKSPACE and foldedProjects is empty", () => {
    const s = useDaemonStore.getState();
    expect(s.selectedWorkspace).toBe(DEFAULT_WORKSPACE);
    expect(s.foldedProjects.size).toBe(0);
  });

  it("setSelectedWorkspace updates selectedWorkspace", () => {
    useDaemonStore.getState().setSelectedWorkspace("prod");
    expect(useDaemonStore.getState().selectedWorkspace).toBe("prod");
  });

  it("toggleProjectFold adds and removes project names from the folded set", () => {
    useDaemonStore.getState().toggleProjectFold("alpha");
    expect(useDaemonStore.getState().foldedProjects.has("alpha")).toBe(true);
    useDaemonStore.getState().toggleProjectFold("alpha");
    expect(useDaemonStore.getState().foldedProjects.has("alpha")).toBe(false);
  });

  it("selectSession follows the picked session's workspace (TUI parity)", () => {
    useDaemonStore.setState({
      sessions: [mkSession("s1", { workspace: "default" }), mkSession("s2", { workspace: "prod" })],
    });
    expect(useDaemonStore.getState().selectedWorkspace).toBe(DEFAULT_WORKSPACE);
    useDaemonStore.getState().selectSession("s2");
    expect(useDaemonStore.getState().selectedWorkspace).toBe("prod");
  });

  it("selectSession(null) leaves selectedWorkspace alone", () => {
    useDaemonStore.setState({ selectedWorkspace: "prod" });
    useDaemonStore.getState().selectSession(null);
    expect(useDaemonStore.getState().selectedWorkspace).toBe("prod");
  });

  it("seedHello follows the daemon's active session into its workspace", () => {
    const frame: HelloFrame = {
      k: "h",
      sessions: [mkSession("s1", { workspace: "default" }), mkSession("s2", { workspace: "prod" })],
      activeSessionID: "s2",
      features: [],
      serverTime: 1,
    };
    useDaemonStore.getState().seedHello(frame);
    expect(useDaemonStore.getState().selectedWorkspace).toBe("prod");
  });

  it("applyViewUpdate resets selectedWorkspace to default when the named workspace disappears", () => {
    useDaemonStore.setState({ selectedWorkspace: "ghost" });
    useDaemonStore.getState().applyViewUpdate({
      k: "v",
      sessions: [mkSession("s1", { workspace: "default" })],
    });
    expect(useDaemonStore.getState().selectedWorkspace).toBe(DEFAULT_WORKSPACE);
  });

  it("applyViewUpdate keeps selectedWorkspace when the workspace still has sessions", () => {
    useDaemonStore.setState({ selectedWorkspace: "prod" });
    useDaemonStore.getState().applyViewUpdate({
      k: "v",
      sessions: [mkSession("s1", { workspace: "prod" })],
    });
    expect(useDaemonStore.getState().selectedWorkspace).toBe("prod");
  });

  // ─── selectors ─────────────────────────────────────────────────────────

  it("selectDistinctWorkspaces always includes DEFAULT_WORKSPACE first", () => {
    expect(selectDistinctWorkspaces([])).toEqual([DEFAULT_WORKSPACE]);
    const ws = selectDistinctWorkspaces([
      mkSession("s1", { workspace: "prod" }),
      mkSession("s2", { workspace: "staging" }),
    ]);
    expect(ws[0]).toBe(DEFAULT_WORKSPACE);
    expect(ws).toEqual([DEFAULT_WORKSPACE, "prod", "staging"]);
  });

  it("projectBasename returns the trailing path segment, stripping trailing slashes", () => {
    expect(projectBasename("/home/me/repos/alpha")).toBe("alpha");
    expect(projectBasename("/home/me/repos/alpha/")).toBe("alpha");
    expect(projectBasename("alpha")).toBe("alpha");
    expect(projectBasename("")).toBe("");
  });

  it("groupSessionsByProject partitions by workspace and groups alphabetical by basename", () => {
    const sessions: SessionInfo[] = [
      mkSession("s1", { project: "/repo/beta", workspace: "default" }),
      mkSession("s2", { project: "/repo/alpha", workspace: "default" }),
      mkSession("s3", { project: "/repo/alpha", workspace: "default" }),
      mkSession("s4", { project: "/repo/gamma", workspace: "prod" }),
    ];
    const groups = groupSessionsByProject(sessions, DEFAULT_WORKSPACE);
    expect(groups.map((g) => g.project)).toEqual(["alpha", "beta"]);
    expect(groups[0]?.sessions.map((s) => s.id)).toEqual(["s2", "s3"]);
    const prodGroups = groupSessionsByProject(sessions, "prod");
    expect(prodGroups.map((g) => g.project)).toEqual(["gamma"]);
  });

  it("groupSessionsByProject keeps distinct paths with the same basename as SEPARATE groups", () => {
    // Regression gate for the basename-collision bug: two repos named "web"
    // under different parents must NOT collapse into a single group, and the
    // projectPath tooltip must point at the right one for each.
    const sessions: SessionInfo[] = [
      mkSession("a", { project: "/home/a/web" }),
      mkSession("b", { project: "/home/b/web" }),
    ];
    const groups = groupSessionsByProject(sessions, DEFAULT_WORKSPACE);
    expect(groups.length).toBe(2);
    expect(groups[0]?.project).toBe("web");
    expect(groups[1]?.project).toBe("web");
    const paths = groups.map((g) => g.projectPath).sort();
    expect(paths).toEqual(["/home/a/web", "/home/b/web"]);
    // Sessions stay with their own path.
    const pathById = new Map(groups.flatMap((g) => g.sessions.map((s) => [s.id, g.projectPath])));
    expect(pathById.get("a")).toBe("/home/a/web");
    expect(pathById.get("b")).toBe("/home/b/web");
  });

  // ─── applyViewUpdate workspace-follow policy ───────────────────────────

  it("applyViewUpdate does NOT overwrite selectedWorkspace on pushes that leave activeSessionID unchanged", () => {
    // Regression gate: previously every view-update reset selectedWorkspace
    // to the active session's workspace, silently undoing a chip click.
    useDaemonStore.setState({
      sessions: [mkSession("s1", { workspace: "default" }), mkSession("s2", { workspace: "prod" })],
      activeSessionID: "s1",
      selectedWorkspace: "prod",
    });
    useDaemonStore.getState().applyViewUpdate({
      k: "v",
      sessions: [mkSession("s1", { workspace: "default" }), mkSession("s2", { workspace: "prod" })],
      // activeSessionID omitted → still s1
    });
    expect(useDaemonStore.getState().selectedWorkspace).toBe("prod");
  });

  it("applyViewUpdate DOES follow the active session when its id changes via the wire", () => {
    useDaemonStore.setState({
      sessions: [mkSession("s1", { workspace: "default" }), mkSession("s2", { workspace: "prod" })],
      activeSessionID: "s1",
      selectedWorkspace: "default",
    });
    useDaemonStore.getState().applyViewUpdate({
      k: "v",
      sessions: [mkSession("s1", { workspace: "default" }), mkSession("s2", { workspace: "prod" })],
      activeSessionID: "s2",
    });
    expect(useDaemonStore.getState().selectedWorkspace).toBe("prod");
  });
});
