import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { Connection } from "./socket/connection";
import { useDaemonStore } from "./store/daemon";
import { useNotificationsStore } from "./store/notifications";

describe("App", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T00:00:00Z"));
    useDaemonStore.getState().reset();
    useNotificationsStore.setState({ items: [] });
    // Stub fetch to hang forever so Connection.start() never rejects and
    // no unhandled rejection leaks out of the voided conn.start() in useEffect.
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => {})),
    );
    // hash token を仕込んで Connection を初期化させる
    window.location.hash = "#token=test";
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    window.location.hash = "";
  });

  it("renders DriverViewPanel for active session", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "Hello driver" }, status: "running" },
        },
      ],
      activeSessionID: "s1",
    });
    render(<App />);
    // Title appears in both SessionList row and DriverViewPanel heading
    const titles = screen.getAllByText("Hello driver");
    expect(titles.length).toBeGreaterThanOrEqual(1);
    // RunStateBadge appears in sidebar and in DriverViewPanel header
    const badges = screen.getAllByLabelText(/status: running/);
    expect(badges.length).toBeGreaterThanOrEqual(1);
    // DriverViewPanel section is rendered
    expect(screen.getByLabelText("driver view")).toBeTruthy();
  });

  it("hides driver view when no active session", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(screen.queryByLabelText("driver view")).toBeNull();
  });

  it("renders MainTabs tablist when active session has log_tabs", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: {
            card: {},
            status: "running",
            log_tabs: [{ label: "Output", path: "/tmp/out.log", kind: "text" }],
          },
        },
      ],
      activeSessionID: "s2",
    });
    render(<App />);
    expect(screen.getByRole("tablist")).toBeTruthy();
    // TERMINAL is prepended as a synthetic tab in front of driver log_tabs.
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["TERMINAL", "Output"]);
  });

  // Regression 2026-06-24: 実 driver (claude_view.go 等) は log_tabs に
  // TRANSCRIPT (path=*.transcript) と EVENTS (path=<sid>.log) を載せる。
  // App は <LogTabs tabs={view.log_tabs}> を render し、両ボタンが見えること
  // を確保する。CSS で潰されたケースは vitest では検知できないが、
  // 「App が LogTabs を render し、tablist に 2 個の [role=tab] が含まれる」
  // ロジック契約はここで永続化する (driver / wire / 描画分岐の regression を防ぐ)。
  it("driver-shaped log_tabs (TRANSCRIPT + EVENTS) renders TERMINAL + both tabs visible to user", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "sess-abc",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: {
            card: { title: "Driver session" },
            status: "running",
            log_tabs: [
              {
                label: "TRANSCRIPT",
                path: "/var/lib/agent-reactor/sess-abc.transcript",
                kind: "text",
              },
              { label: "EVENTS", path: "/var/log/agent-reactor/sess-abc.log", kind: "text" },
            ],
          },
        },
      ],
      activeSessionID: "sess-abc",
    });
    render(<App />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(3);
    expect(tabs.map((t) => t.textContent)).toEqual(["TERMINAL", "TRANSCRIPT", "EVENTS"]);
  });

  // Regression 2026-06-24: suppress_info が真でないとき、空でない log_tabs は
  // 必ず render されること (App.tsx の条件分岐回帰防止)。MainTabs 化以後は
  // 常に TERMINAL タブが先頭に乗るため tab 数 = 1 + driver log_tabs.length。
  it("does NOT hide LogTabs when suppress_info is unset/false", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s3",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: {
            card: {},
            status: "running",
            suppress_info: false,
            log_tabs: [{ label: "EVENTS", path: "/x/s3.log", kind: "text" }],
          },
        },
      ],
      activeSessionID: "s3",
    });
    render(<App />);
    expect(screen.queryByRole("tablist")).not.toBeNull();
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["TERMINAL", "EVENTS"]);
  });

  it("renders NotificationToast aria-label container", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(screen.getByLabelText("notifications")).toBeTruthy();
  });

  it("keyed remount: switching activeSessionID causes TerminalPane to remount", () => {
    // Start with session s1 active
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "running" },
        },
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "running" },
        },
      ],
      activeSessionID: "s1",
    });
    const { rerender } = render(<App />);

    // Capture the terminal-host element from first render
    const hostBefore = document.querySelector(".terminal-host");
    expect(hostBefore).not.toBeNull();

    // Switch to s2 — this changes the key prop on TerminalPane, forcing remount
    act(() => {
      useDaemonStore.setState({ activeSessionID: "s2" });
    });
    rerender(<App />);

    // After remount the .terminal-host element is a fresh DOM node
    const hostAfter = document.querySelector(".terminal-host");
    expect(hostAfter).not.toBeNull();
    // The key change means React unmounts old and mounts new — DOM node differs
    expect(hostAfter).not.toBe(hostBefore);
  });

  it("ADR 0030: keyed remount unsubscribes old session and subscribes new one", () => {
    // Spy on the Connection prototype so we capture calls made by whatever
    // Connection instance App's useMemo allocates internally. unsubscribe is
    // stubbed to a no-op resolved promise; subscribe is stubbed to never
    // resolve so the awaited retry loop inside the real implementation does
    // not run (we only need to observe that the method was invoked with the
    // expected sessionId).
    const subscribeSpy = vi
      .spyOn(Connection.prototype, "subscribe")
      .mockImplementation(() => new Promise<void>(() => {}));
    const unsubscribeSpy = vi
      .spyOn(Connection.prototype, "unsubscribe")
      .mockImplementation(async () => {});

    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "running" },
        },
        {
          id: "s2",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "running" },
        },
      ],
      activeSessionID: "s1",
    });
    const { rerender } = render(<App />);

    // Initial mount: keyed TerminalPane subscribes to s1; nothing to unsubscribe.
    expect(subscribeSpy).toHaveBeenCalledWith("s1");
    expect(unsubscribeSpy).not.toHaveBeenCalled();

    subscribeSpy.mockClear();

    // Switch active session — old TerminalPane unmounts (→ unsubscribe('s1')),
    // new TerminalPane mounts under the new key (→ subscribe('s2')).
    act(() => {
      useDaemonStore.setState({ activeSessionID: "s2" });
    });
    rerender(<App />);

    expect(unsubscribeSpy).toHaveBeenCalledWith("s1");
    expect(subscribeSpy).toHaveBeenCalledWith("s2");
  });
});
