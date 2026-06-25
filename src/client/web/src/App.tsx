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
// isMacPlatform centralizes the Cmd/Ctrl-key label decision (FR-D3 / ADR-0048).
// The previous local isMac() was a duplicate of the single-source helper in
// lib/platform.ts; routing through the lib lets the hotkey hook and Header
// share the same UA-Client-Hints / navigator.platform / userAgent fallback
// chain (FR-D1 / FR-D2 — no crash when navigator is undefined).
import { isMacPlatform } from "./lib/platform";
import { Connection } from "./socket/connection";
import { useDaemonStore } from "./store/daemon";
import { useNotificationsStore } from "./store/notifications";
import { usePaletteStore } from "./store/palette";
import { useDaemonSnapshot } from "./store/useDaemonSnapshot";

export function App() {
  // ADR-0037 / FR-001: intercept Cmd/Ctrl+K on the capture phase.
  // Invariant: mount this exactly once across the App tree. Do not call from
  // multiple sites.
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
          message: `Failed to load session config: ${msg}`,
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

  // Subscribe to the daemon primitives that feed selectDaemonSnapshot so the
  // Header re-renders whenever a field consumed by the snapshot changes — same
  // pattern as ParamSelectPhase (Y3 single source). We pass the
  // snapshot through openPalette({daemonSnapshot}) on the New Session CTA
  // (FR-A2) so the store can resolve preselectToolId='new-session' against the
  // live daemon (push-aware) instead of falling back to an empty snapshot.
  const daemonSnapshot = useDaemonSnapshot();

  return (
    <div className="app">
      <NotificationToast />
      <StatusBanner />
      <header className="app-header">
        {/* FR-002: the always-on Command button is a fallback for the
            Cmd/Ctrl+K hotkey (ADR-0037). It stays visible in the Header so
            the palette can always be opened even on environments where the
            hotkey is consumed by a browser-native binding. f2: after
            CreateSessionForm was removed, the New Session button was folded
            into this Header and opens the palette pre-filtered to 'new'
            (ADR-0043 / FR-021). */}
        <button
          type="button"
          aria-label="Open command palette (⌘K / Ctrl+K)"
          onClick={(e) =>
            usePaletteStore.getState().openPalette({ opener: e.currentTarget, daemonSnapshot })
          }
        >
          Command ({isMacPlatform() ? "⌘K" : "Ctrl+K"})
        </button>
        {/* FR-021 / ADR-0043: the legacy CreateSessionForm "New Session"
            CTA is replaced by palette new-session. openPalette({
            preselectToolId }) surfaces the palette, skips ToolSelectPhase,
            and lands directly on the new-session paramSelect phase (the
            invariant of pinning to one tool by ID is expressed in the
            palette-store layer). We do not go through the fuzzy filter
            because newSessionTool.label was historically Japanese ("New
            session" rendered as JP) — a "new" / "new-session" query would
            return 0 hits. store-level preselect resolves by ToolDef.id
            equality so it is independent of label localization. */}
        <button
          type="button"
          aria-label="New Session"
          onClick={(e) => {
            usePaletteStore.getState().openPalette({
              opener: e.currentTarget,
              preselectToolId: "new-session",
              // FR-A2: pass the live daemon snapshot so palette-store can
              // resolve preselect against the actual occupant / projects and
              // normalize scope to 'standard' instead of falling back to an
              // empty snapshot (which would silently swap behavior if a
              // future tool changed its disabledReason).
              daemonSnapshot,
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
      {/* Mounted via portal directly under <body>, so the placement of
          this element in the tree is irrelevant (ADR-0036). */}
      <CommandPalette />
    </div>
  );
}
