import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { Connection } from "./socket/connection";
import { useDaemonStore } from "./store/daemon";
import { useNotificationsStore } from "./store/notifications";
import { usePaletteStore } from "./store/palette";

describe("App", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T00:00:00Z"));
    useDaemonStore.getState().reset();
    useNotificationsStore.setState({ items: [] });
    // FR-002 / FR-001: Header の Command ボタンと useGlobalHotkey() は
    // usePaletteStore に書き込むため、テスト間で open=true の漏れを防ぐ。
    usePaletteStore.getState().close();
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
    usePaletteStore.getState().close();
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

  // FR-002 / ADR-0037: 常設 Command ボタンは Cmd/Ctrl+K の保険。
  // クリックで palette store が open=true になり、opener にボタン自身が
  // セットされていること (CommandPalette の focus 復元先になる)。
  it("Header の Command ボタンクリックで palette が open になる (FR-002)", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    const btn = screen.getByLabelText("Command Palette");
    expect(btn).toBeTruthy();
    act(() => {
      fireEvent.click(btn);
    });

    const s = usePaletteStore.getState();
    expect(s.open).toBe(true);
    // opener はクリック元のボタンが入る
    expect(s.opener).toBe(btn);
  });

  // FR-001 / ADR-0037: App は useGlobalHotkey() を 1 回 mount し、
  // document の capture-phase で Cmd+K (mac) / Ctrl+K (other) を拾う。
  it("Cmd+K (mac) で palette が open になる — useGlobalHotkey 配線 (FR-001)", () => {
    // navigator.platform を mac に偽装
    const platformSpy = vi.spyOn(navigator, "platform", "get").mockReturnValue("MacIntel");
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      fireEvent.keyDown(document, { key: "k", metaKey: true });
    });

    expect(usePaletteStore.getState().open).toBe(true);
    platformSpy.mockRestore();
  });

  it("Ctrl+K (non-mac) で palette が open になる — useGlobalHotkey 配線 (FR-001)", () => {
    const platformSpy = vi.spyOn(navigator, "platform", "get").mockReturnValue("Linux x86_64");
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      fireEvent.keyDown(document, { key: "k", ctrlKey: true });
    });

    expect(usePaletteStore.getState().open).toBe(true);
    platformSpy.mockRestore();
  });

  it("非対象キーでは palette は開かない (regression guard)", () => {
    const platformSpy = vi.spyOn(navigator, "platform", "get").mockReturnValue("MacIntel");
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      // metaKey 無しの k は palette を開かない
      fireEvent.keyDown(document, { key: "k" });
    });
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      // 別キーは palette を開かない
      fireEvent.keyDown(document, { key: "j", metaKey: true });
    });
    expect(usePaletteStore.getState().open).toBe(false);

    platformSpy.mockRestore();
  });

  // FR-002: ボタン label は platform に合わせて切り替わる
  it("Command ボタンの label が mac / non-mac で切り替わる", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });

    const macSpy = vi.spyOn(navigator, "platform", "get").mockReturnValue("MacIntel");
    const { unmount } = render(<App />);
    expect(screen.getByLabelText("Command Palette").textContent).toContain("⌘K");
    unmount();
    macSpy.mockRestore();

    const linuxSpy = vi.spyOn(navigator, "platform", "get").mockReturnValue("Linux x86_64");
    render(<App />);
    expect(screen.getByLabelText("Command Palette").textContent).toContain("Ctrl+K");
    linuxSpy.mockRestore();
  });

  // FR-021 / ADR-0043 (f2): 旧 CreateSessionForm の "New Session" CTA は
  // palette new-session に置換された。Header の New Session ボタンクリックで
  //   1) palette が open になる
  //   2) opener が当該ボタンに set される (focus 復元先)
  //   3) ToolSelectPhase をスキップして new-session の paramSelect phase に
  //      直接進む (selectedToolId='new-session' / phase='paramSelect')
  //   ID で 1 件に固定する不変条件は palette-store の preselectToolId 経由で
  //   表現する。fuzzy label 検索ではない理由は newSessionTool.label が
  //   日本語 ("新しいセッション") のため "new" 系の query で 0 hit になるため
  //   (review 指摘 #1 の blocker 回帰防止)。
  it("Header の New Session ボタンクリックで palette が new-session paramSelect phase で開く (FR-021)", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    const btn = screen.getByLabelText("New Session");
    expect(btn).toBeTruthy();
    act(() => {
      fireEvent.click(btn);
    });

    const s = usePaletteStore.getState();
    expect(s.open).toBe(true);
    expect(s.opener).toBe(btn);
    // ID ベースの不変条件: ToolSelectPhase はスキップされ、new-session 1 件に
    // 固定された paramSelect phase に進む。query / fuzzy 結果には依存しない。
    expect(s.phase).toBe("paramSelect");
    expect(s.selectedToolId).toBe("new-session");
  });

  // Blocker T1 regression guard: App on mount MUST call
  // GET /api/session-config and feed the result to
  // useDaemonStore.setSessionConfig. Without this, ParamSelectPhase /
  // ScopeSegment see empty projects + pushCommands forever (the production
  // code path otherwise never fires session-config-extension's REST hydrate).
  it("mount で GET /api/session-config を叩き、結果を daemon.setSessionConfig に渡す (T1)", async () => {
    // Replace the default hang-forever fetch stub with one that resolves
    // the session-config call with a representative payload, and any
    // other call (ws-ticket) with a never-resolving promise so Connection
    // does not blow up the test.
    const fetchSpy = vi.fn((url: RequestInfo | URL) => {
      const u = typeof url === "string" ? url : url.toString();
      if (u.endsWith("/api/session-config")) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              commands: ["claude"],
              projects: [{ path: "/repo/a", isGit: true, isSandboxed: false }],
              push_commands: ["/clear", "/exit"],
            }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
        );
      }
      return new Promise(() => {}); // hang ws-ticket forever
    });
    vi.stubGlobal("fetch", fetchSpy);

    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);

    // Wait one microtask tick for the promise chain to flush. We do this
    // by running pending timers + awaiting several resolved promises so
    // vitest's fakeTimers settings (set in beforeEach) do not stall the
    // await chain that fetch -> request -> text() -> setSessionConfig walks.
    await vi.runOnlyPendingTimersAsync().catch(() => {});
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        await Promise.resolve();
      });
    }

    const calls = fetchSpy.mock.calls.filter((c) => {
      const u = typeof c[0] === "string" ? c[0] : (c[0] as URL).toString();
      return u.endsWith("/api/session-config");
    });
    expect(calls.length).toBeGreaterThanOrEqual(1);
    const cfg = useDaemonStore.getState().sessionConfig;
    expect(cfg).not.toBeNull();
    expect(cfg?.projects).toEqual([{ path: "/repo/a", isGit: true, isSandboxed: false }]);
    expect(cfg?.pushCommands).toEqual(["/clear", "/exit"]);
  });

  // T1 follow-up: on getSessionConfig failure (non-401) we surface a
  // single error toast and leave sessionConfig=null. 401 is silenced
  // because Connection.start owns the auth-error UX.
  it("mount の getSessionConfig が失敗したら error toast を出す (T1 失敗パス)", async () => {
    const fetchSpy = vi.fn((url: RequestInfo | URL) => {
      const u = typeof url === "string" ? url : url.toString();
      if (u.endsWith("/api/session-config")) {
        return Promise.resolve(new Response("daemon down", { status: 503 }));
      }
      return new Promise(() => {});
    });
    vi.stubGlobal("fetch", fetchSpy);

    render(<App />);
    // Flush the promise chain without advancing fake timers: the
    // NotificationToast auto-dismisses after 5000ms via setTimeout, and
    // runOnlyPendingTimersAsync would fire that immediately, wiping the
    // very item we are about to assert on. Microtask flush via
    // act(Promise.resolve()) walks fetch -> request -> text() -> throw
    // -> catch -> useNotificationsStore.add without touching timers.
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        await Promise.resolve();
      });
    }

    const items = useNotificationsStore.getState().items;
    const errors = items.filter((i) => i.level === "error");
    expect(errors.length).toBeGreaterThanOrEqual(1);
    expect(useDaemonStore.getState().sessionConfig).toBeNull();
  });

  // f2 regression guard: 旧 CreateSessionForm の form / dialog はもう
  // render されない (撤去済み)。Project directory input / Create ボタン /
  // dialog 要素のいずれも DOM に出ないこと。
  it("旧 CreateSessionForm の form 要素 (Project directory / Create) は render されない (f2 撤去)", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(screen.queryByLabelText("Project directory")).toBeNull();
    expect(screen.queryByRole("button", { name: /^Create$/ })).toBeNull();
  });
});
