// Canonical JSON fixtures matching Go wire_test.go byte-for-byte where applicable.
// OutputFrame: [timeSec, "o", data] — Go wire.go:18 json.Marshal([]any{e.TimeSec,"o",string(data)})
// ControlFrame: {"k":"c","code":<int omitempty>,"data":<string omitempty>}
//   code=0 is omitted by omitempty (see gateway.go:104: controlFrame("c",0,"daemon-disconnected"))
//   code≠0 appears as integer (see wire_test.go:57: Code:9)
export const fixtures = {
  output: '[1.5,"o","hi"]',
  control: '{"k":"c","data":"daemon-disconnected"}',
  controlWithCode: '{"k":"c","code":9,"data":"t | b"}',
  hello:
    '{"k":"h","sessions":[{"id":"s1","title":"t1","status":"running","createdAt":1700000000}],"activeSessionID":"s1","features":["surface"],"serverTime":1700000001}',
  viewUpdate:
    '{"k":"v","sessions":[{"id":"s1","title":"t1","status":"running","createdAt":1700000000}],"activeSessionID":"s1"}',
  respOK: '{"k":"r","reqId":"req-1","body":{"ok":true}}',
  respErr: '{"k":"e","reqId":"req-2","code":"frame-not-ready","message":"not yet"}',
} as const;
