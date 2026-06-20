// Server → browser wire frames. Mirror of Go side (do not change wire shape).
// Surface output uses asciicast v2 tuple form: [eventCode, timeSec, dataB64].
// Control / hello / view-update / resp use JSON object form with discriminator `k`.

export type SessionInfo = {
  id: string;
  title: string;
  status: string; // "running" | "stopped" | "errored" など server 側 string をそのまま保持
  createdAt: number;
};

// asciicast v2: 配列形式 [eventCode, timeSec, dataB64]
// eventCode は現状 "o"(output)のみ使用。"i"(input)は将来予約。
export type OutputFrame = ["o", number, string];

export type ControlFrame = {
  k: "c";
  code: string; // "daemon-disconnected" | "slow-subscriber" など
  data?: string | string[];
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
