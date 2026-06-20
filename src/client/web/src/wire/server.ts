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

// asciicast-style array [timeSec, type, dataB64] — Go wire.go と同順。
// type は現状 "o"(output)のみ使用。dataB64 は base64 文字列(NOT raw bytes):
// daemon の DataB64 をそのまま wire に乗せ、ブラウザ側 TerminalPane が atob
// で復号して Uint8Array で xterm.write に渡す。json.Marshal が非 UTF-8 バイトを
// U+FFFD で破壊する問題(2026-06-20 code-review round 3)の対策。
export type OutputFrame = [number, "o", string];

// ControlFrame mirrors Go controlMsg{K,Code int omitempty,Data string omitempty}.
// code=0 is omitted by Go's omitempty, so code is optional here.
// data carries event-specific payload (e.g. "daemon-disconnected").
export type ControlFrame = {
  k: "c";
  code?: number; // int, omitempty — absent when 0
  data?: string; // omitempty
};

export type HelloFrame = {
  k: "h";
  sessions: SessionInfo[];
  activeSessionID: string | null;
  features: string[];
  serverTime: number;
  connectors?: ConnectorInfo[];
};

export type ViewUpdateFrame = {
  k: "v";
  sessions: SessionInfo[];
  activeSessionID?: string | null;
  connectors?: ConnectorInfo[];
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

// ConnectorItem mirrors view.ConnectorItem — one entry within a ConnectorSection.
export type ConnectorItem = {
  symbol: string;
  title: string;
  meta: string;
};

// ConnectorSection mirrors view.ConnectorSection — titled group of items.
export type ConnectorSection = {
  title: string;
  items?: ConnectorItem[];
};

// ConnectorInfo mirrors proto.ConnectorInfo — per-connector wire payload.
export type ConnectorInfo = {
  name: string;
  label: string;
  summary: string;
  available: boolean;
  sections?: ConnectorSection[];
};

export type ConnectorUpdateFrame = {
  k: "cu";
  connectors: ConnectorInfo[];
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
  | ConnectorUpdateFrame
  | RespOKFrame
  | RespErrFrame;
