import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
// FR-D1 / FR-D2 / FR-D3: Header's Cmd/Ctrl-K label routes through the
// lib/platform single-source helper instead of the deleted local isMac().
// We mock the lib so each test can flip mac / non-mac without touching
// navigator (which different envs surface differently — userAgentData on
// Chromium, deprecated navigator.platform on Safari/Firefox, etc).
import { isMacPlatform } from "./lib/platform";
import { Connection } from "./socket/connection";
import { selectDaemonSnapshot, useDaemonStore } from "./store/daemon";
import { useNotificationsStore } from "./store/notifications";
import { usePaletteStore } from "./store/palette";
import { mkSnapshot } from "./test/fixtures/daemon";

vi.mock("./lib/platform", () => ({
  isMacPlatform: vi.fn(),
}));

describe("App", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T00:00:00Z"));
    useDaemonStore.getState().reset();
    useNotificationsStore.setState({ items: [] });
    // FR-002 / FR-001: Header の Command ボタンと useGlobalHotkey() は
    // usePaletteStore に書き込むため、テスト間で open=true の漏れを防ぐ。
    usePaletteStore.getState().close();
    // Default isMacPlatform → false (Linux). Mac-branch tests override per case.
    vi.mocked(isMacPlatform).mockReturnValue(false);
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

    const btn = screen.getByLabelText("Open command palette (⌘K / Ctrl+K)");
    expect(btn).toBeTruthy();
    act(() => {
      fireEvent.click(btn);
    });

    const s = usePaletteStore.getState();
    expect(s.open).toBe(true);
    // opener はクリック元のボタンが入る
    expect(s.opener).toBe(btn);
  });

  // FR-001 / ADR-0037: App mounts useGlobalHotkey() once and listens on the
  // document capture phase for Cmd+K (mac) / Ctrl+K (other).
  //
  // useGlobalHotkey reads isMacPlatform() from ./lib/platform — the SAME mocked
  // module the test importer sees. Spying navigator.platform alone is NOT enough
  // because the module-level vi.mock above replaces the implementation with a
  // vi.fn(); we have to flip the mock return per case via vi.mocked(...).
  it("Cmd+K (mac) opens palette — useGlobalHotkey wiring (FR-001)", () => {
    vi.mocked(isMacPlatform).mockReturnValue(true);
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      fireEvent.keyDown(document, { key: "k", metaKey: true });
    });

    expect(usePaletteStore.getState().open).toBe(true);
  });

  it("Ctrl+K (non-mac) opens palette — useGlobalHotkey wiring (FR-001)", () => {
    vi.mocked(isMacPlatform).mockReturnValue(false);
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      fireEvent.keyDown(document, { key: "k", ctrlKey: true });
    });

    expect(usePaletteStore.getState().open).toBe(true);
  });

  it("non-target keys do not open palette (regression guard)", () => {
    vi.mocked(isMacPlatform).mockReturnValue(true);
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    render(<App />);
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      // k without metaKey must not open the palette
      fireEvent.keyDown(document, { key: "k" });
    });
    expect(usePaletteStore.getState().open).toBe(false);

    act(() => {
      // Another key with metaKey must not open the palette
      fireEvent.keyDown(document, { key: "j", metaKey: true });
    });
    expect(usePaletteStore.getState().open).toBe(false);
  });

  // FR-D1 / FR-D2 / FR-D3: Command ボタンの label / aria-label は
  // lib/platform:isMacPlatform を一次ソースとして mac / non-mac で切り替わる。
  // Header はもう navigator.platform を直に読まない (旧 isMac() 削除済み) — ので
  // 各分岐は vi.mocked(isMacPlatform) を直接フリップして driver する。
  it("FR-D1: Header button shows ⌘K when isMacPlatform()=true and exposes the hotkey-bearing aria-label", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    vi.mocked(isMacPlatform).mockReturnValue(true);
    render(<App />);
    const btn = screen.getByLabelText("Open command palette (⌘K / Ctrl+K)");
    expect(btn.textContent).toContain("⌘K");
    expect(btn.getAttribute("aria-label")).toBe("Open command palette (⌘K / Ctrl+K)");
  });

  it("FR-D2: Header button shows Ctrl+K when isMacPlatform()=false (no crash on fallback envs)", () => {
    useDaemonStore.setState({ sessions: [], activeSessionID: null });
    vi.mocked(isMacPlatform).mockReturnValue(false);
    render(<App />);
    const btn = screen.getByLabelText("Open command palette (⌘K / Ctrl+K)");
    expect(btn.textContent).toContain("Ctrl+K");
    expect(btn.textContent).not.toContain("⌘K");
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

  // FR-A2: Header の New Session ボタン onClick は openPalette に
  // {preselectToolId:'new-session', daemonSnapshot, opener} を必ず渡す。
  // daemonSnapshot は selectDaemonSnapshot 経由で実 store の値を渡すので、
  // palette-store 側で preselect 解決時に scope='standard' へ正規化されつつ
  // 実 daemon (push 占有 / projects) を見て disabledReason / 後段の materialize
  // が正しく動く (空 snapshot fallback 経路に落ちない)。
  it("FR-A2: Header New Session click calls openPalette with {preselectToolId, daemonSnapshot, opener}", () => {
    // 実 daemon に projects + pushCommands を seed して mkSnapshot と同形の
    // snapshot が流れることを確認する。
    useDaemonStore.setState({
      sessions: [],
      activeSessionID: null,
      sessionConfig: {
        projects: [{ path: "/repo1", isGit: true, isSandboxed: false }],
        pushCommands: ["/clear"],
      },
    });
    const expectedSnapshot = mkSnapshot({
      projects: [{ path: "/repo1", isGit: true, isSandboxed: false }],
      pushCommands: ["/clear"],
    });
    // selectDaemonSnapshot は store/daemon の単一ソース。test 側でも同じ
    // 関数で組み立てて、App 側が渡す snapshot と等値であることを assert する。
    const liveSnapshot = selectDaemonSnapshot(useDaemonStore.getState());
    expect(liveSnapshot).toEqual(expectedSnapshot);

    const openSpy = vi.spyOn(usePaletteStore.getState(), "openPalette");

    render(<App />);
    const btn = screen.getByLabelText("New Session");
    act(() => {
      fireEvent.click(btn);
    });

    expect(openSpy).toHaveBeenCalledTimes(1);
    const arg = openSpy.mock.calls[0]?.[0];
    expect(arg).toBeDefined();
    expect(arg?.preselectToolId).toBe("new-session");
    expect(arg?.opener).toBe(btn);
    // daemonSnapshot は selectDaemonSnapshot の結果 (mkSnapshot で組んだ
    // canonical 形と等値)。
    expect(arg?.daemonSnapshot).toEqual(expectedSnapshot);

    openSpy.mockRestore();
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
    // English-only gate: session-config 失敗 toast は英語に置換 (旧
    // "session-config の取得に失敗しました:" は撤去)。Server message は
    // ": <reason>" の形で連結されたまま末尾に残る。
    expect(errors[0]?.message).toMatch(/^Failed to load session config:/);
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
