// Browser → server wire frames. Sent as JSON over WebSocket.
//
// The lifecycle WS multiplexes input and resize across many sessions, so
// InputFrame and ResizeFrame both carry the target sessionId. The Go side
// (server/web/gateway.go::readLifecycleInbound) drops frames whose
// `sessionId` is empty.

export type InputFrame = {
  k: "i";
  d: string; // raw input string. server converts to CmdSurfaceWriteRaw{Data:[]byte(d)}
  sessionId: string;
};

export type ResizeFrame = {
  k: "r";
  cols: number;
  rows: number;
  sessionId: string;
};

export type SubscribeFrame = {
  k: "s";
  reqId: string;
  sessionId: string;
};

export type UnsubscribeFrame = {
  k: "u";
  reqId: string;
  sessionId: string;
};

export type ClientFrame = InputFrame | ResizeFrame | SubscribeFrame | UnsubscribeFrame;
