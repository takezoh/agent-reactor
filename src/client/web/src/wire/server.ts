// Server → browser wire frames. Mirror of Go side (do not change wire shape).
// Surface output uses asciicast v2 tuple form: [timeSec, "o", data].
// Control / hello / view-update / resp use JSON object form with discriminator `k`.

// Tag mirrors view.Tag — colored chip in session card.
export type Tag = {
  text: string;
  fg?: string;
  bg?: string;
};

// Card mirrors view.Card — driver-specific portion of the session list card.
export type Card = {
  title?: string;
  subtitle?: string;
  tags?: Tag[];
  border_title?: Tag;
  border_title_secondary?: Tag;
  border_badge?: string;
};

// LogTab mirrors view.LogTab — additional log tab declared by the driver.
export type LogTab = {
  label: string;
  path: string;
  kind: string; // "text" etc. — Go's TabKind is a string alias
  renderer_cfg?: unknown; // json.RawMessage
};

// InfoLine mirrors view.InfoLine — one entry in the INFO tab body.
export type InfoLine = {
  label: string;
  value: string;
};

// View mirrors view.View — complete TUI payload for one session.
export type View = {
  card: Card;
  display_name?: string;
  log_tabs?: LogTab[];
  info_extras?: InfoLine[];
  suppress_info?: boolean;
  status_line?: string;
  status?: string; // "running" | "waiting" | "idle" | "stopped" | "pending"
  status_changed_at?: string; // RFC3339
};

// SessionInfo mirrors proto.SessionInfo — Go JSON tags used as-is (snake_case).
export type SessionInfo = {
  id: string;
  project: string;
  workspace?: string;
  command: string;
  root_driver?: string;
  root_driver_forkable?: boolean;
  created_at: string; // RFC3339 string (Go: time.Time formatted)
  state?: string;
  state_changed_at?: string;
  view: View;
  is_active?: boolean;
};

// asciicast-style array [timeSec, type, dataB64, sessionId] — Go wire.go と同順。
// type は現状 "o"(output)のみ使用。dataB64 は base64 文字列(NOT raw bytes):
// daemon の DataB64 をそのまま wire に乗せ、ブラウザ側 TerminalPane が atob
// で復号して Uint8Array で xterm.write に渡す。json.Marshal が非 UTF-8 バイトを
// U+FFFD で破壊する問題(2026-06-20 code-review round 3)の対策。
// 4 番目の sessionId は AttachLifecycleWS の多重化 routing 用(round 4 finding):
// session 切替時の subscribe/unsubscribe overlap で前 session の出力が新 session
// の terminal に漏れ込むのを TerminalPane 側で filter する。
export type OutputFrame = [number, "o", string, string];

// ControlFrame mirrors Go controlMsg{K,Code int omitempty,Data string omitempty}.
// code=0 is omitted by Go's omitempty, so code is optional here.
// data carries event-specific payload (e.g. "daemon-disconnected").
export type ControlFrame = {
  k: "c";
  code?: number; // int, omitempty — absent when 0
  data?: string; // omitempty
};

// ActiveOccupant mirrors the daemon-side OccupantKind: which buffer occupies
// pane 0.1 in the TUI ("main" | "log" | "frame"). Carried daemon-globally
// (NOT per session — ADR-0044) so the web palette can gate the push scope
// (FR-005/FR-006). Optional / omitempty on the Go side: an absent value
// reads as "no frame" via the existing fail-closed path.
export type ActiveOccupant = "main" | "log" | "frame";

export type HelloFrame = {
  k: "h";
  sessions: SessionInfo[];
  activeSessionID: string | null;
  // Optional for backward compat: pre-this-change servers do not emit the
  // field. The TS reducer in store/daemon.ts treats `undefined` as "leave
  // the current value alone" so a clean reconnect after a binary upgrade
  // does not regress to the pre-2026-06-24 fail-closed default.
  activeOccupant?: ActiveOccupant;
  features: string[];
  serverTime: number;
};

export type ViewUpdateFrame = {
  k: "v";
  sessions: SessionInfo[];
  activeSessionID?: string | null;
  // See HelloFrame.activeOccupant. View-update frames carry it live so a
  // frame pushed / popped by another driver client toggles push availability
  // in the palette without requiring a reconnect (the daemon broadcasts
  // EvtSessionsChanged on occupant changes).
  activeOccupant?: ActiveOccupant;
};

export type TranscriptTailFrame = {
  k: "tt";
  sessionId: string;
  line: string;
};

export type EventLogTailFrame = {
  k: "et";
  sessionId: string;
  line: string;
};

export type NotificationFrame = {
  k: "n";
  sessionId: string;
  cmd: number;
  title?: string;
  body?: string;
  nowMs: number;
};

export type RespOKFrame = {
  k: "r";
  reqId: string;
  body?: unknown;
};

export type RespErrFrame = {
  k: "e";
  reqId: string;
  code: string; // "frame-not-ready" | "unauthorized" | ...
  message: string;
};

export type ServerFrame =
  | OutputFrame
  | ControlFrame
  | HelloFrame
  | ViewUpdateFrame
  | TranscriptTailFrame
  | EventLogTailFrame
  | NotificationFrame
  | RespOKFrame
  | RespErrFrame;
