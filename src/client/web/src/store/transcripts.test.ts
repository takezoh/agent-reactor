import { beforeEach, describe, expect, it } from "vitest";
import { selectBuffer, useTranscriptStore } from "./transcripts";

describe("useTranscriptStore", () => {
  beforeEach(() => {
    useTranscriptStore.getState().reset();
  });

  it("TestAppendLineAddsToBuffer: appendLine stores the line with restOffset 0", () => {
    useTranscriptStore.getState().appendLine("s1", "transcript", "hello");
    const buf = selectBuffer(useTranscriptStore.getState(), "s1", "transcript");
    expect(buf).toBeDefined();
    expect(buf?.lines).toEqual(["hello"]);
    expect(buf?.restOffset).toBe(0);
  });

  it("TestAppendLineRingsAt1000: 1001 lines → length 1000, head is line2", () => {
    for (let i = 1; i <= 1001; i++) {
      useTranscriptStore.getState().appendLine("s1", "transcript", `line${i}`);
    }
    const buf = selectBuffer(useTranscriptStore.getState(), "s1", "transcript");
    expect(buf?.lines).toHaveLength(1000);
    // line1 was evicted; first kept is line2
    expect(buf?.lines[0]).toBe("line2");
    expect(buf?.lines[999]).toBe("line1001");
  });

  it("TestAppendBackfillSetsOffsetAndLines: backfill replaces with merged lines and updates restOffset", () => {
    useTranscriptStore.getState().appendBackfill("s1", "transcript", ["a", "b"], 42);
    const buf = selectBuffer(useTranscriptStore.getState(), "s1", "transcript");
    expect(buf?.lines).toEqual(["a", "b"]);
    expect(buf?.restOffset).toBe(42);
  });

  it("TestAppendBackfillPreservesExistingTail: existing 5 lines + backfill 5 = 10 lines total, REST first", () => {
    // The WS tail can arrive before the REST response (the React session-
    // select dispatches both in parallel). REST backfill is historically
    // OLDER than anything already in the buffer, so appendBackfill prepends
    // its lines to the buffer rather than appending — verifies the
    // chronological order documented in transcripts.ts::appendBackfill.
    for (let i = 1; i <= 5; i++) {
      useTranscriptStore.getState().appendLine("s1", "transcript", `tail${i}`);
    }
    useTranscriptStore
      .getState()
      .appendBackfill("s1", "transcript", ["bf1", "bf2", "bf3", "bf4", "bf5"], 99);
    const buf = selectBuffer(useTranscriptStore.getState(), "s1", "transcript");
    expect(buf?.lines).toHaveLength(10);
    expect(buf?.lines[0]).toBe("bf1");
    expect(buf?.lines[4]).toBe("bf5");
    expect(buf?.lines[5]).toBe("tail1");
    expect(buf?.lines[9]).toBe("tail5");
    expect(buf?.restOffset).toBe(99);
  });

  it("TestSeparateKeysPerKind: transcript and event-log buffers are independent", () => {
    useTranscriptStore.getState().appendLine("s1", "transcript", "t-line");
    useTranscriptStore.getState().appendLine("s1", "event-log", "e-line");
    const tBuf = selectBuffer(useTranscriptStore.getState(), "s1", "transcript");
    const eBuf = selectBuffer(useTranscriptStore.getState(), "s1", "event-log");
    expect(tBuf?.lines).toEqual(["t-line"]);
    expect(eBuf?.lines).toEqual(["e-line"]);
  });

  it("TestClearSessionRemovesBothKinds: clearSession removes transcript and event-log for that session", () => {
    useTranscriptStore.getState().appendLine("s1", "transcript", "x");
    useTranscriptStore.getState().appendLine("s1", "event-log", "y");
    useTranscriptStore.getState().appendLine("s2", "transcript", "z");
    useTranscriptStore.getState().clearSession("s1");
    expect(selectBuffer(useTranscriptStore.getState(), "s1", "transcript")).toBeUndefined();
    expect(selectBuffer(useTranscriptStore.getState(), "s1", "event-log")).toBeUndefined();
    // s2 is unaffected
    expect(selectBuffer(useTranscriptStore.getState(), "s2", "transcript")?.lines).toEqual(["z"]);
  });

  it("TestResetEmptiesAll: reset clears all buffers", () => {
    useTranscriptStore.getState().appendLine("s1", "transcript", "a");
    useTranscriptStore.getState().appendLine("s2", "event-log", "b");
    useTranscriptStore.getState().reset();
    expect(useTranscriptStore.getState().buffers).toEqual({});
  });
});
