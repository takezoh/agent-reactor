import { useEffect, useMemo } from "react";
import { makeSessionsApi } from "./api/sessions";
import type { ApiHttpError } from "./api/sessions";
import { readBearerTokenFromHash } from "./auth";
import { DriverViewPanel } from "./components/DriverViewPanel";
import { MainTabs } from "./components/MainTabs";
import { NotificationToast } from "./components/NotificationToast";
import { SessionList } from "./components/SessionList";
import { StatusBanner } from "./components/StatusBanner";
import { TerminalPane } from "./components/TerminalPane";
import { CommandPalette } from "./components/palette/CommandPalette";
import { useGlobalHotkey } from "./hooks/useGlobalHotkey";
import { Connection } from "./socket/connection";
import { useDaemonStore } from "./store/daemon";
import { useNotificationsStore } from "./store/notifications";
import { usePaletteStore } from "./store/palette";

// isMac is a UI-label helper: the global hotkey hook does its own platform
// detection (see useGlobalHotkey / ADR-0037). Centralizing both in one helper
// would risk drift since the hook needs SSR-safety while this only renders
// in the browser — keeping them separate is the cheaper invariant.
function isMac(): boolean {
  if (typeof navigator === "undefined") return false;
  return navigator.platform.toUpperCase().includes("MAC");
}

export function App() {
  // ADR-0037 / FR-001: capture-phase で Cmd/Ctrl+K を横取りする。
  // App ツリー全体で 1 回だけ mount する不変条件。複数箇所で呼ばないこと。
  useGlobalHotkey();

  const token = useMemo(() => readBearerTokenFromHash(), []);
  const conn = useMemo(
    () =>
      new Connection({
        ticketEndpoint: "/api/ws-ticket",
        wsUrl: (ticket) => {
          const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
          return `${proto}//${window.location.host}/ws?ticket=${encodeURIComponent(ticket)}`;
        },
        bearerToken: token,
      }),
    [token],
  );

  useEffect(() => {
    void conn.start();
    return () => conn.close();
  }, [conn]);

  // Blocker T1: hydrate daemon.sessionConfig at mount so the command
  // palette has projects + pushCommands available. Without this call,
  // GET /api/session-config never fires from production code path and
  // ParamSelectPhase / scope gating see empty lists forever (FR-013 /
  // FR-014 toggles never light up, push scope stays fail-closed).
  //
  // - We swallow 401 silently: the auth bootstrap (Connection.start)
  //   handles the missing-token path with its own toast; surfacing it
  //   twice would be noisy.
  // - Other HTTP / network failures surface as a single error toast and
  //   leave sessionConfig=null. The CommandPalette already logs a
  //   breadcrumb when sessionConfig is missing at open-time, so the
  //   diagnostic chain stays intact.
  // - We deliberately do NOT block UI rendering on this fetch: the
  //   palette can still surface the standard-scope tools (new-session,
  //   stop-session) with empty project lists while the request is in
  //   flight; the hydrate fires-and-fills as soon as it lands.
  useEffect(() => {
    const api = makeSessionsApi();
    let cancelled = false;
    api
      .getSessionConfig()
      .then((cfg) => {
        if (cancelled) return;
        useDaemonStore.getState().setSessionConfig(cfg);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        // 401 = token missing / stale. Connection.start surfaces this
        // separately; we stay quiet here so the user does not see two
        // toasts for the same auth gap.
        if (e instanceof Error && (e as ApiHttpError).status === 401) {
          return;
        }
        const msg = e instanceof Error ? e.message : String(e);
        useNotificationsStore.getState().add({
          level: "error",
          message: `session-config の取得に失敗しました: ${msg}`,
        });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const activeSession = useDaemonStore((s) =>
    s.activeSessionID ? (s.sessions.find((x) => x.id === s.activeSessionID) ?? null) : null,
  );

  return (
    <div className="app">
      <NotificationToast />
      <StatusBanner />
      <header className="app-header">
        {/* FR-002: 常設 Command ボタンは Cmd/Ctrl+K の hotkey 保険 (ADR-0037)。
            hotkey が browser-native binding に奪われる環境でも palette を必ず
            開けるよう、Header に視認可能なまま残す。f2: CreateSessionForm 撤去後
            は New Session ボタンもこの Header に統合され、palette を 'new'
            で pre-filter して開く (ADR-0043 / FR-021)。 */}
        <button
          type="button"
          aria-label="Command Palette"
          onClick={(e) => usePaletteStore.getState().openPalette({ opener: e.currentTarget })}
        >
          Command ({isMac() ? "⌘K" : "Ctrl+K"})
        </button>
        {/* FR-021 / ADR-0043: 旧 CreateSessionForm の "New Session" CTA を
            palette new-session に置換。openPalette({preselectToolId}) で
            palette を出し、ToolSelectPhase をスキップして直接 new-session の
            paramSelect phase に進む (ID で 1 件に固定する不変条件を palette-
            store 層で表現する)。fuzzy filter 経由ではない理由は newSessionTool
            の label が日本語 ("新しいセッション") のため "new" / "new-session"
            のいずれの query でも 0 hit になるから。store-level preselect は
            ToolDef.id の同値で resolve するので日本語 label の影響を受けない。 */}
        <button
          type="button"
          aria-label="New Session"
          onClick={(e) => {
            usePaletteStore.getState().openPalette({
              opener: e.currentTarget,
              preselectToolId: "new-session",
            });
          }}
        >
          New Session
        </button>
      </header>
      <aside className="sidebar">
        <SessionList conn={conn} />
      </aside>
      <main className="terminal">
        {activeSession && <DriverViewPanel view={activeSession.view} />}
        <MainTabs
          tabs={activeSession?.view.log_tabs ?? []}
          sessionId={activeSession?.id}
          bearerToken={token}
          suppressInfo={activeSession?.view.suppress_info ?? false}
          terminalSlot={
            <TerminalPane
              key={activeSessionID ?? "__none__"}
              conn={conn}
              sessionId={activeSessionID}
            />
          }
        />
      </main>
      {/* portal で body 直下に出るので mount 位置は任意 (ADR-0036)。 */}
      <CommandPalette />
    </div>
  );
}
