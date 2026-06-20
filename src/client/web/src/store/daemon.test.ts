import { beforeEach, describe, expect, it } from "vitest";
import type { HelloFrame, ViewUpdateFrame } from "../wire/server";
import { useDaemonStore } from "./daemon";

describe("daemonStore", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
  });

  it("seedHello populates sessions/features/serverTime/activeSessionID", () => {
    const frame: HelloFrame = {
      k: "h",
      sessions: [{ id: "s1", title: "t", status: "running", createdAt: 100 }],
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
      sessions: [{ id: "s2", title: "t2", status: "stopped", createdAt: 200 }],
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
});
