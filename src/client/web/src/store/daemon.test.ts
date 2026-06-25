import { beforeEach, describe, expect, it } from "vitest";
import type { SessionConfig } from "../api/sessions";
import type { HelloFrame, SessionInfo, ViewUpdateFrame } from "../wire/server";
import { selectDaemonSnapshot, useDaemonStore } from "./daemon";

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
});
