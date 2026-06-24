import { useEffect, useMemo } from "react";
import { readBearerTokenFromHash } from "./auth";
import { ConnectorPanel } from "./components/ConnectorPanel";
import { CreateSessionForm } from "./components/CreateSessionForm";
import { DriverViewPanel } from "./components/DriverViewPanel";
import { MainTabs } from "./components/MainTabs";
import { NotificationToast } from "./components/NotificationToast";
import { SessionList } from "./components/SessionList";
import { StatusBanner } from "./components/StatusBanner";
import { TerminalPane } from "./components/TerminalPane";
import { Connection } from "./socket/connection";
import { useDaemonStore } from "./store/daemon";

export function App() {
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

  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const activeSession = useDaemonStore((s) =>
    s.activeSessionID ? (s.sessions.find((x) => x.id === s.activeSessionID) ?? null) : null,
  );

  return (
    <div className="app">
      <NotificationToast />
      <StatusBanner />
      <aside className="sidebar">
        <CreateSessionForm conn={conn} bearerToken={token} />
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
      <ConnectorPanel />
    </div>
  );
}
