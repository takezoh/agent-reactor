import { useEffect, useMemo } from "react";
import { readBearerTokenFromHash } from "./auth";
import { CreateSessionForm } from "./components/CreateSessionForm";
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

  return (
    <div className="app">
      <StatusBanner />
      <aside className="sidebar">
        <CreateSessionForm conn={conn} />
        <SessionList conn={conn} />
      </aside>
      <main className="terminal">
        <TerminalPane conn={conn} sessionId={activeSessionID} />
      </main>
    </div>
  );
}
