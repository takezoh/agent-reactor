// Browser → server wire frames. Sent as JSON over WebSocket.

export type InputFrame = {
  k: "i";
  d: string; // raw input string. server converts to CmdSurfaceWriteRaw{Data:[]byte(d)}
};

export type ResizeFrame = {
  k: "r";
  cols: number;
  rows: number;
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
