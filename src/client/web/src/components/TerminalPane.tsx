import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import { useEffect, useRef } from "react";
import "@xterm/xterm/css/xterm.css";
import type { Connection } from "../socket/connection";
import { useXtermTheme } from "./ThemeProvider";

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

  // Keep the latest sessionId in a ref so the xterm.js onData / onResize
  // handlers always read the current active session without re-binding (the
  // outer useEffect intentionally only runs once per Connection instance).
  const sessionRef = useRef<string | null>(sessionId);
  sessionRef.current = sessionId;

  // FR-THEME-003 (ADR-0059): ITheme derived from CSS tokens via ThemeProvider.
  // Rebuilds whenever data-theme changes (1 rAF guard inside useXtermTheme).
  const xtermTheme = useXtermTheme();

  // Ref to the live Terminal instance so the theme effect can reach it without
  // being included in the main lifecycle dependency array.
  const termRef = useRef<Terminal | null>(null);

  // Stable ref to the latest xtermTheme so the main lifecycle effect can read
  // the current value at construction time without xtermTheme appearing in its
  // dependency array (which would re-run the heavy setup on every theme change).
  const xtermThemeRef = useRef(xtermTheme);
  xtermThemeRef.current = xtermTheme;

  // FR-THEME-002: apply new ITheme to the live terminal on every theme rebuild.
  // termRef.current is null until the main lifecycle effect mounts the terminal.
  // Initial ITheme application is guaranteed by the main lifecycle effect below,
  // which passes xtermTheme directly to new Terminal({ theme }) and also assigns
  // term.options.theme immediately after term.open(host). This effect handles
  // subsequent data-theme changes after mount; the early-out here is the normal
  // pre-mount no-op (not a silent fallback — initial application is explicit).
  useEffect(() => {
    const term = termRef.current;
    if (!term) return; // pre-mount: initial theme applied in main lifecycle below
    term.options.theme = xtermTheme;
  }, [xtermTheme]);

  // Main lifecycle: create terminal, attach to host, wire conn.onOutput,
  // window resize, and ResizeObserver. Runs once per (conn) mount.
  // ADR 0030: keyed remount via <TerminalPane key={activeSessionID}> in
  // App.tsx ensures a fresh TerminalPane instance for each session switch —
  // no session-clear effect is needed here.
  useEffect(() => {
    if (!hostRef.current) return;
    const host = hostRef.current;
    // Pass xtermThemeRef.current at construction time so the very first painted
    // frame uses the correct token-derived palette. Without this, xterm adopts
    // its built-in white-background defaults for the first render cycle before
    // the theme-apply effect above runs (FR-THEME-002 / ADR-0059).
    // We use the ref rather than the reactive value to keep [conn] as the sole
    // dependency — re-creating the terminal on every theme change would be wrong.
    // scrollback must be ≥ the server-side termvt scrollback cap
    // (settings.toml [terminal] scrollback_lines, default 10000 — ADR-0066).
    // Otherwise the seed frame for a late-joining client carries lines the
    // browser silently discards before the user can scroll them into view.
    const term = new Terminal({ convertEol: true, scrollback: 10000, theme: xtermThemeRef.current });
    termRef.current = term;
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);

    // ADR 0034: scheduleFit coalesces rapid resize events into a single rAF
    // tick. A pending flag ensures at most one rAF is queued at a time so
    // consecutive ResizeObserver / window-resize firings do not fan out.
    let rafPending = false;
    function scheduleFit() {
      if (rafPending) return;
      rafPending = true;
      requestAnimationFrame(() => {
        rafPending = false;
        fit.fit();
      });
    }

    // Initial fit via scheduleFit so the first fit also runs in a rAF tick
    // (avoids 0-size measurement on initial paint, satisfies FR-008).
    scheduleFit();

    const onData = term.onData((d) => {
      const sid = sessionRef.current;
      if (!sid) return; // no session selected → drop
      conn.send({ k: "i", d, sessionId: sid });
    });
    const onResize = term.onResize(({ cols, rows }) => {
      const sid = sessionRef.current;
      if (!sid) return;
      conn.send({ k: "r", cols, rows, sessionId: sid });
    });

    conn.onOutput = (frame) => {
      // The Go wire (server/web/wire.go:outputFrameFromSurface) sends
      // [TimeSec, "o", DataB64, SessionID]. DataB64 is the base64 STRING
      // (NOT the decoded bytes). Decoding to a Go string and JSON-marshalling
      // raw PTY bytes is unsafe — encoding/json silently replaces non-UTF-8
      // bytes with U+FFFD, garbling 256-color sequences and any non-ASCII
      // output. atob → Uint8Array preserves every byte.
      //
      // Filter by sessionId: AttachLifecycleWS multiplexes surface output
      // for every subscribed session. During session switch the unsubscribe
      // for the previous session is fire-and-forget, so its output may keep
      // arriving briefly after the new session subscribe lands. Without the
      // sessionId filter that stale output bleeds into the new terminal.
      if (frame[3] !== sessionRef.current) return;
      term.write(b64ToBytes(frame[2]));
    };

    // window resize → scheduleFit
    const handleWindowResize = () => scheduleFit();
    window.addEventListener("resize", handleWindowResize);

    // ResizeObserver on host element → scheduleFit (ADR 0034, FR-006/007)
    const ro = new ResizeObserver(() => scheduleFit());
    ro.observe(host);

    return () => {
      window.removeEventListener("resize", handleWindowResize);
      ro.disconnect();
      onData.dispose();
      onResize.dispose();
      conn.onOutput = undefined;
      term.dispose();
      termRef.current = null;
    };
  }, [conn]);

  // Subscribe ownership: TerminalPane is the sole owner of subscribe/unsubscribe
  // for sessionId. keyed remount via App.tsx means each session gets a fresh
  // instance; the cleanup here unsubscribes when the component unmounts.
  // (ADR 0030: SessionList.tsx must not call subscribe/unsubscribe — see
  // session-list-label-and-subscribe task.)
  useEffect(() => {
    if (!sessionId) return;
    void conn.subscribe(sessionId);
    return () => {
      void conn.unsubscribe(sessionId);
    };
  }, [conn, sessionId]);

  return <div ref={hostRef} className="terminal-host" />;
}
