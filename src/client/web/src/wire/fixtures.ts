export const fixtures = {
  output: '["o",1.234,"aGVsbG8="]',
  control: '{"k":"c","code":"daemon-disconnected"}',
  controlWithData: '{"k":"c","code":"slow-subscriber","data":["s1","s2"]}',
  hello:
    '{"k":"h","sessions":[{"id":"s1","title":"t1","status":"running","createdAt":1700000000}],"activeSessionID":"s1","features":["surface"],"serverTime":1700000001}',
  viewUpdate:
    '{"k":"v","sessions":[{"id":"s1","title":"t1","status":"running","createdAt":1700000000}],"activeSessionID":"s1"}',
  respOK: '{"k":"r","reqId":"req-1","body":{"ok":true}}',
  respErr: '{"k":"e","reqId":"req-2","code":"frame-not-ready","message":"not yet"}',
} as const;
