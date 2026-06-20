import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import { useEffect, useRef } from "react";
import "@xterm/xterm/css/xterm.css";
import type { Connection } from "../socket/connection";

// b64ToBytes decodes a base64 string into a Uint8Array. atob() returns a
// binary string whose char codes are the raw byte values; copying them into
// a Uint8Array gives xterm.js the byte-faithful payload it needs (xterm
// accepts string | Uint8Array; Uint8Array bypasses the UTF-8 decoder).
function b64ToBytes(b64: string): Uint8Array {
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
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

  // Keep the latest sessionId in a ref so the xterm.js onData / onResize
  // handlers always read the current active session without re-binding (the
  // outer useEffect intentionally only runs once per Connection instance).
  const sessionRef = useRef<string | null>(sessionId);
  sessionRef.current = sessionId;

  useEffect(() => {
    if (!hostRef.current) return;
    const term = new Terminal({ convertEol: true, scrollback: 5000 });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(hostRef.current);
    fit.fit();
    termRef.current = term;

    const onData = term.onData((d) => {
      const sid = sessionRef.current;
      if (!sid) return; // no session selected → drop, do not send empty sessionId
      conn.send({ k: "i", d, sessionId: sid });
    });
    const onResize = term.onResize(({ cols, rows }) => {
      const sid = sessionRef.current;
      if (!sid) return;
      conn.send({ k: "r", cols, rows, sessionId: sid });
    });

    conn.onOutput = (frame) => {
      // The Go wire (server/web/wire.go:outputFrameFromSurface) sends
      // [TimeSec, "o", DataB64] where the third element is the base64
      // STRING (NOT the decoded bytes). Decoding to a Go string and
      // JSON-marshalling raw PTY bytes is unsafe — encoding/json silently
      // replaces non-UTF-8 bytes with U+FFFD, garbling 256-color sequences
      // and any non-ASCII output. atob → Uint8Array preserves every byte.
      term.write(b64ToBytes(frame[2]));
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
