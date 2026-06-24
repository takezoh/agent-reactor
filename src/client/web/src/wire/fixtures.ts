// Canonical JSON fixtures matching Go wire_test.go byte-for-byte where applicable.
// OutputFrame: [timeSec, "o", dataB64, sessionId] — Go wire.go outputFrameFromSurface
// ControlFrame: {"k":"c","code":<int omitempty>,"data":<string omitempty>}
//   code=0 is omitted by omitempty (see gateway.go:104: controlFrame("c",0,"daemon-disconnected"))
//   code≠0 appears as integer (see wire_test.go:57: Code:9)
export const fixtures = {
  output: '[1.5,"o","hi","s1"]',
  control: '{"k":"c","data":"daemon-disconnected"}',
  controlWithCode: '{"k":"c","code":9,"data":"t | b"}',
  hello:
    '{"k":"h","sessions":[{"id":"s1","project":"p","command":"claude","created_at":"2026-06-20T00:00:00Z","view":{"card":{"title":"T","subtitle":"S","tags":[{"text":"tag"}]},"status":"running","status_changed_at":"2026-06-20T00:00:00Z","status_line":"thinking","log_tabs":[{"label":"events","path":"/tmp/x","kind":"text"}]}}],"activeSessionID":"s1","features":["surface"],"serverTime":1700000001}',
  // activeSessionID is intentionally absent from view-update fixtures —
  // the Go gateway drops it from the wire so that daemon-side ActiveSession
  // mutations (every spawn / frame push) do not clobber each browser's
  // locally-tracked selection. See server/web/wire.go viewUpdateFrame doc.
  viewUpdate:
    '{"k":"v","sessions":[{"id":"s1","project":"p","command":"claude","created_at":"2026-06-20T00:00:00Z","view":{"card":{"title":"T2"},"status":"idle"}}]}',
  respOK: '{"k":"r","reqId":"req-1","body":{"ok":true}}',
  respErr: '{"k":"e","reqId":"req-2","code":"frame-not-ready","message":"not yet"}',
  logTabsTranscriptFrame: '{"k":"tt","sessionId":"s1","line":"[claude] hello world"}',
  notificationFrame:
    '{"k":"n","sessionId":"s1","cmd":9,"title":"Task done","body":"Session finished","nowMs":1700000002000}',
} as const;
