import { describe, expect, it } from "vitest";
import { parseServerFrame, serializeClientFrame } from "./codec";
import { fixtures } from "./fixtures";

describe("parseServerFrame", () => {
  it.each(Object.entries(fixtures))("parses %s round-trip", (_name, raw) => {
    const parsed = parseServerFrame(raw);
    expect(parsed).not.toBeNull();
    // serialize back: output frame is array, others are objects
    const re = Array.isArray(parsed) ? JSON.stringify(parsed) : JSON.stringify(parsed);
    const reparsed = parseServerFrame(re);
    expect(reparsed).toEqual(parsed);
  });

  it("returns null for invalid JSON", () => {
    expect(parseServerFrame("not json")).toBeNull();
  });

  it("returns null for unknown discriminator", () => {
    expect(parseServerFrame('{"k":"xyz"}')).toBeNull();
  });

  it("returns null for missing discriminator", () => {
    expect(parseServerFrame("{}")).toBeNull();
  });

  it("returns null for malformed output array", () => {
    // [timeSec, "o", dataB64, sessionId] order — too short (length 3 is the
    // pre-round-4 wire and must be rejected to surface mismatch loudly)
    expect(parseServerFrame('[1.0,"o","data"]')).toBeNull();
    // length-2 also rejected
    expect(parseServerFrame('[1.0,"o"]')).toBeNull();
    // timeSec must be number
    expect(parseServerFrame('["not-number","o","data","s1"]')).toBeNull();
    // sessionId must be string (not number)
    expect(parseServerFrame('[1.0,"o","data",42]')).toBeNull();
    // old wrong order ["o", number, string, string] must also return null
    expect(parseServerFrame('["o",1.0,"data","s1"]')).toBeNull();
  });

  it("parses view-update with full View payload", () => {
    const frame = parseServerFrame(fixtures.viewUpdate);
    expect(frame).not.toBeNull();
    if (!frame || Array.isArray(frame) || frame.k !== "v") throw new Error("expected v frame");
    const sess = frame.sessions[0];
    expect(sess).toBeDefined();
    expect(sess?.view.card.title).toBe("T2");
    expect(sess?.view.status).toBe("idle");
  });

  it("returns null when sessions[].view is missing", () => {
    expect(parseServerFrame('{"k":"v","sessions":[{"id":"s1"}]}')).toBeNull();
  });

  it("view-update without activeSessionID leaves field undefined (preserve semantics)", () => {
    // Go's omitempty strips empty active_session_id from the wire. The store
    // relies on `activeSessionID === undefined` to mean "no change" — coercing
    // to null would clobber the web client's local selection on every daemon
    // broadcast.
    const raw =
      '{"k":"v","sessions":[{"id":"s1","view":{"card":{"title":"T"},"status":"idle"}}]}';
    const parsed = parseServerFrame(raw);
    if (!parsed || Array.isArray(parsed) || parsed.k !== "v") throw new Error("expected v frame");
    expect(parsed.activeSessionID).toBeUndefined();
    expect("activeSessionID" in parsed).toBe(false);
  });

  it("view-update with explicit null activeSessionID forwards null (override)", () => {
    const raw =
      '{"k":"v","sessions":[{"id":"s1","view":{"card":{"title":"T"},"status":"idle"}}],"activeSessionID":null}';
    const parsed = parseServerFrame(raw);
    if (!parsed || Array.isArray(parsed) || parsed.k !== "v") throw new Error("expected v frame");
    expect(parsed.activeSessionID).toBeNull();
  });

  it("view-update with string activeSessionID forwards string", () => {
    const raw =
      '{"k":"v","sessions":[{"id":"s1","view":{"card":{"title":"T"},"status":"idle"}}],"activeSessionID":"s1"}';
    const parsed = parseServerFrame(raw);
    if (!parsed || Array.isArray(parsed) || parsed.k !== "v") throw new Error("expected v frame");
    expect(parsed.activeSessionID).toBe("s1");
  });

  it("returns null when sessions[].view.card is missing", () => {
    expect(
      parseServerFrame('{"k":"v","sessions":[{"id":"s1","view":{"status":"idle"}}]}'),
    ).toBeNull();
  });

  it('parses "tt" round-trip', () => {
    const raw = '{"k":"tt","sessionId":"s1","line":"hi"}';
    const parsed = parseServerFrame(raw);
    expect(parsed).not.toBeNull();
    expect(parsed).toEqual({ k: "tt", sessionId: "s1", line: "hi" });
    expect(parseServerFrame(JSON.stringify(parsed))).toEqual(parsed);
  });

  it('parses "et" round-trip', () => {
    const raw = '{"k":"et","sessionId":"s1","line":"event line"}';
    const parsed = parseServerFrame(raw);
    expect(parsed).not.toBeNull();
    expect(parsed).toEqual({ k: "et", sessionId: "s1", line: "event line" });
    expect(parseServerFrame(JSON.stringify(parsed))).toEqual(parsed);
  });

  it('rejects "tt" missing sessionId', () => {
    expect(parseServerFrame('{"k":"tt","line":"hi"}')).toBeNull();
  });

  it('parses "n" round-trip', () => {
    const raw = '{"k":"n","sessionId":"s1","cmd":9,"title":"t","body":"b","nowMs":123}';
    const parsed = parseServerFrame(raw);
    expect(parsed).not.toBeNull();
    expect(parsed).toEqual({ k: "n", sessionId: "s1", cmd: 9, title: "t", body: "b", nowMs: 123 });
    expect(parseServerFrame(JSON.stringify(parsed))).toEqual(parsed);
  });

  it('rejects "n" missing nowMs', () => {
    expect(parseServerFrame('{"k":"n","sessionId":"s1","cmd":9}')).toBeNull();
  });

  it('parses "cu" round-trip', () => {
    const raw =
      '{"k":"cu","connectors":[{"name":"github","label":"GitHub","summary":"","available":true}]}';
    const parsed = parseServerFrame(raw);
    expect(parsed).not.toBeNull();
    expect(parsed).toEqual({
      k: "cu",
      connectors: [{ name: "github", label: "GitHub", summary: "", available: true }],
    });
    expect(parseServerFrame(JSON.stringify(parsed))).toEqual(parsed);
  });

  it('rejects "cu" with malformed connector (available not boolean)', () => {
    expect(
      parseServerFrame(
        '{"k":"cu","connectors":[{"name":"github","label":"GitHub","summary":"","available":"yes"}]}',
      ),
    ).toBeNull();
  });

  it('parses "h" with connectors', () => {
    const raw =
      '{"k":"h","sessions":[],"activeSessionID":null,"features":[],"serverTime":1,"connectors":[{"name":"gh","label":"GitHub","summary":"ok","available":true}]}';
    const parsed = parseServerFrame(raw);
    expect(parsed).not.toBeNull();
    if (!parsed || Array.isArray(parsed) || parsed.k !== "h") throw new Error("expected h frame");
    expect(parsed.connectors).toEqual([
      { name: "gh", label: "GitHub", summary: "ok", available: true },
    ]);
  });
});

describe("serializeClientFrame", () => {
  it("serializes input frame", () => {
    expect(serializeClientFrame({ k: "i", d: "abc", sessionId: "s1" })).toBe(
      '{"k":"i","d":"abc","sessionId":"s1"}',
    );
  });
  it("serializes resize frame", () => {
    expect(serializeClientFrame({ k: "r", cols: 80, rows: 24, sessionId: "s1" })).toBe(
      '{"k":"r","cols":80,"rows":24,"sessionId":"s1"}',
    );
  });
  it("serializes subscribe frame", () => {
    expect(serializeClientFrame({ k: "s", reqId: "r1", sessionId: "s1" })).toBe(
      '{"k":"s","reqId":"r1","sessionId":"s1"}',
    );
  });
});
