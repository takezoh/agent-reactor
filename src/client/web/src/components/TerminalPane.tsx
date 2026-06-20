import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import { useEffect, useRef } from "react";
import "@xterm/xterm/css/xterm.css";
import type { Connection } from "../socket/connection";

function b64decode(d: string): string {
  // browser native atob handles base64; xterm.write accepts string or Uint8Array
  return atob(d);
}

export function TerminalPane({
  conn,
  sessionId,
}: {
  conn: Connection;
  sessionId: string | null;
}) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<Terminal | null>(null);

  useEffect(() => {
    if (!hostRef.current) return;
    const term = new Terminal({ convertEol: true, scrollback: 5000 });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(hostRef.current);
    fit.fit();
    termRef.current = term;

    const onData = term.onData((d) => {
      conn.send({ k: "i", d });
    });
    const onResize = term.onResize(({ cols, rows }) => {
      conn.send({ k: "r", cols, rows });
    });

    conn.onOutput = (frame) => {
      // frame = ["o", timeSec, dataB64]
      const dataB64 = frame[2];
      term.write(b64decode(dataB64));
    };

    const handleResize = () => fit.fit();
    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      onData.dispose();
      onResize.dispose();
      conn.onOutput = undefined;
      term.dispose();
      termRef.current = null;
    };
  }, [conn]);

  // when sessionId changes, we don't reset xterm — TerminalPane is keyed on
  // sessionId by parent if a full reset is desired. β: single shared term.
  useEffect(() => {
    if (!sessionId) return;
    void conn.subscribe(sessionId);
    return () => {
      void conn.unsubscribe(sessionId);
    };
  }, [conn, sessionId]);

  return <div ref={hostRef} className="terminal-host" />;
}
