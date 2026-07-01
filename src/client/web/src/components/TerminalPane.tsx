import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import { useEffect, useRef, useState } from "react";
import "@xterm/xterm/css/xterm.css";
import { useMobileGate } from "../hooks/useMobileGate";
import type { Connection } from "../socket/connection";
import { useDaemonStore } from "../store/daemon";
import { TerminalMobileOverlay } from "./TerminalMobileOverlay";
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

// handleNewlineModifier inspects a raw KeyboardEvent and, when the user pressed
// Shift+Enter or Alt+Enter, forwards a literal newline escape (`\` + CR) to the
// PTY and tells xterm.js to skip its default Enter handling. The Claude CLI
// family (claude code etc.) is the primary consumer of this terminal and
// treats `\<CR>` as "insert newline" instead of "submit", which mirrors the
// in-terminal escape the user already knows. Returning false here suppresses
// xterm's onData fire for this keystroke so the bare CR does not double-submit.
//
// IME composition is skipped: e.isComposing is true while the user is mid-
// conversion (typically Japanese / Chinese / Korean IMEs). A Shift+Enter that
// merely confirms the composition must not flush `\<CR>` to the PTY — let the
// IME's own Enter handling run.
//
// We also call e.preventDefault() before returning false. attachCustomKeyEventHandler
// only controls xterm.js's internal keymap; without preventDefault the browser
// default for Enter (a literal newline appended to the .xterm-helper-textarea
// value) can leak. xterm normally preventDefaults inside its own onKey path,
// so skipping that path means we own the suppression.
//
// Returns the value xterm.js's attachCustomKeyEventHandler expects: true to
// keep default processing, false to swallow the event.
export function handleNewlineModifier(e: KeyboardEvent, sendInput: (d: string) => void): boolean {
  if (e.type !== "keydown") return true;
  if (e.key !== "Enter") return true;
  if (e.isComposing) return true;
  if (!e.shiftKey && !e.altKey) return true;
  e.preventDefault();
  sendInput("\\\r");
  return false;
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

  // [terminal] font_family / font_size from settings.toml, delivered over
  // GET /api/session-config (ADR-0041) and cached in the daemon store. Empty
  // string / 0 mean "unset" — we then pass no font option to xterm.js so its
  // built-in monospace default is preserved. Same ref pattern as the theme:
  // read at construction time, and re-apply on change via the effect below,
  // so a late-landing session-config fetch does not re-run the heavy lifecycle.
  const fontFamily = useDaemonStore((s) => s.sessionConfig?.fontFamily ?? "");
  const fontSize = useDaemonStore((s) => s.sessionConfig?.fontSize ?? 0);
  const fontRef = useRef({ family: fontFamily, size: fontSize });
  fontRef.current = { family: fontFamily, size: fontSize };

  // ─── Mobile overlay wiring (ADR 0069 / 0072 / 0074) ──────────────────────
  // These refs / signals are populated by the main lifecycle effect after the
  // terminal is opened, then handed to the conditionally-mounted mobile overlay.
  // On the PC path the overlay is never rendered, so none of this perturbs the
  // legacy terminal-host (FR-PC-PRESERVE-*): the gate is the single render
  // switch (ADR 0067), not a CSS display:none.
  const viewportRef = useRef<HTMLElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const scheduleFitRef = useRef<() => void>(() => {});
  const [termReady, setTermReady] = useState(false);
  const [seedReady, setSeedReady] = useState(false);
  const mobile = useMobileGate();

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

  // Apply [terminal] font_family / font_size to the live terminal whenever the
  // cached session-config changes (e.g. the initial REST fetch lands after the
  // terminal already mounted, or the user edits settings.toml and reconnects).
  // Changing the font alters the character cell size, so re-fit afterwards to
  // recompute cols/rows. Empty / zero are treated as "keep current" so an
  // unset config never blanks the grid font. Mirrors the theme effect above;
  // the pre-mount null case is a normal no-op (construction applies the ref).
  useEffect(() => {
    const term = termRef.current;
    if (!term) return;
    if (fontFamily) term.options.fontFamily = fontFamily;
    if (fontSize > 0) term.options.fontSize = fontSize;
    scheduleFitRef.current();
  }, [fontFamily, fontSize]);

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
    // Only pass font options when configured: an empty fontFamily / zero
    // fontSize would override xterm.js's built-in default with a blank value.
    const font = fontRef.current;
    const term = new Terminal({
      convertEol: true,
      scrollback: 10000,
      theme: xtermThemeRef.current,
      ...(font.family ? { fontFamily: font.family } : {}),
      ...(font.size > 0 ? { fontSize: font.size } : {}),
    });
    termRef.current = term;
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);

    // ADR 0069 mobile wiring: resolve the xterm-built children the mobile hooks
    // attach to. On PC the overlay never mounts so these refs are simply unused.
    viewportRef.current = host.querySelector<HTMLElement>(".xterm-viewport");
    textareaRef.current = host.querySelector<HTMLTextAreaElement>(".xterm-helper-textarea");

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
    // Expose the single ADR-0034 scheduleFit to the mobile fontSize / pinch
    // paths so every refit funnels through one rAF-coalesced queue.
    scheduleFitRef.current = scheduleFit;

    // Initial fit via scheduleFit so the first fit also runs in a rAF tick
    // (avoids 0-size measurement on initial paint, satisfies FR-008).
    scheduleFit();

    // The terminal DOM now exists; allow the mobile overlay to mount and let its
    // hooks bind to the resolved viewport / helper textarea (gate-true only).
    setTermReady(true);

    const onData = term.onData((d) => {
      const sid = sessionRef.current;
      if (!sid) return; // no session selected → drop
      conn.send({ k: "i", d, sessionId: sid });
    });
    // Shift+Enter / Alt+Enter inject a literal newline (Claude CLI's `\<CR>`
    // escape) directly, so the user no longer has to type `\` then Enter.
    // Runs before xterm's own keymap, so returning false here swallows the
    // bare-CR onData that would otherwise submit the prompt.
    term.attachCustomKeyEventHandler((e) =>
      handleNewlineModifier(e, (d) => {
        const sid = sessionRef.current;
        if (!sid) return;
        conn.send({ k: "i", d, sessionId: sid });
      }),
    );
    const onResize = term.onResize(({ cols, rows }) => {
      const sid = sessionRef.current;
      if (!sid) return;
      conn.send({ k: "r", cols, rows, sessionId: sid });
    });

    // ADR 0066 seed-flush signal: the first output frame is the scrollback seed;
    // until it lands the jump-to-latest FAB stays forced-absent (FR-MOB-JUMP-005).
    let seeded = false;

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
      if (!seeded) {
        seeded = true;
        setSeedReady(true);
      }
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
      viewportRef.current = null;
      textareaRef.current = null;
      setTermReady(false);
      setSeedReady(false);
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

  // PC path: render exactly the legacy terminal-host (FR-PC-PRESERVE-*). The
  // mobile overlay is a conditionally-mounted *sibling* of terminal-host (the
  // ADR-0069 `.terminal-fab-layer` absolute overlay anchored to terminal-slot),
  // NOT a child of the xterm-managed host (which would collide with xterm's
  // imperative DOM). When the gate is false the fragment renders only
  // terminal-host — DOM-identical to the legacy `<div className="terminal-host"/>`
  // (ADR 0067 gate is the single render switch; no display:none, no PC change).
  return (
    <>
      <div ref={hostRef} className="terminal-host" />
      {mobile && termReady && (
        <TerminalMobileOverlay
          hostRef={hostRef}
          termRef={termRef}
          viewportRef={viewportRef}
          textareaRef={textareaRef}
          scheduleFit={() => scheduleFitRef.current()}
          seedReady={seedReady}
          sendInput={(d) => {
            // ADR 0077: mirrors the `term.onData` guard above — drop input if
            // no session is active, otherwise forward as one wire frame.
            const sid = sessionRef.current;
            if (!sid) return;
            conn.send({ k: "i", d, sessionId: sid });
          }}
        />
      )}
    </>
  );
}
