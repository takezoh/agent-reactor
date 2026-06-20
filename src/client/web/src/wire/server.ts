// Server → browser wire frames. Mirror of Go side (do not change wire shape).
// Surface output uses asciicast v2 tuple form: [timeSec, "o", data].
// Control / hello / view-update / resp use JSON object form with discriminator `k`.

export type SessionInfo = {
  id: string;
  title: string;
  status: string; // "running" | "stopped" | "errored" など server 側 string をそのまま保持
  createdAt: number;
};

// asciicast v2: 配列形式 [timeSec, type, data] — Go wire.go:18 と同順。
// type は現状 "o"(output)のみ使用。
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
};

export type ViewUpdateFrame = {
  k: "v";
  sessions: SessionInfo[];
  activeSessionID?: string | null;
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
  | RespOKFrame
  | RespErrFrame;
