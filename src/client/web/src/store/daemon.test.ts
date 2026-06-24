import { beforeEach, describe, expect, it } from "vitest";
import type { HelloFrame, SessionInfo, ViewUpdateFrame } from "../wire/server";
import { useDaemonStore } from "./daemon";

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
});
