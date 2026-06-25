// UAC-004 / UAC-006 / UAC-014 / FR-001 / FR-002 / FR-026
// Tests for sortToolsForList and resolveCursorBySelectedToolId.

import { describe, expect, it, vi } from "vitest";
import type { DaemonSnapshot, ToolDef } from "../lib/tools";
import {
  type SortedToolEntry,
  resolveCursorBySelectedToolId,
  sortToolsForList,
} from "./palette_helpers";

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

function makeDaemon(): DaemonSnapshot {
  return {
    sessions: [],
    activeSessionID: null,
    projects: [],
    pushCommands: [],
  };
}

function makeTool(
  id: string,
  disabledReason: (daemon: DaemonSnapshot) => string | null = () => null,
): ToolDef {
  return {
    id,
    label: id,
    scope: "standard",
    params: null,
    disabledReason,
    submit: async () => {},
  };
}

// A minimal FuzzyHit-like fixture: { item: ToolDef }
function hit(tool: ToolDef): { item: ToolDef } {
  return { item: tool };
}

// ---------------------------------------------------------------------------
// sortToolsForList
// ---------------------------------------------------------------------------

describe("sortToolsForList", () => {
  it("returns empty sorted when fuzzyRanked is empty", () => {
    const result = sortToolsForList([], makeDaemon());
    expect(result.enabled).toHaveLength(0);
    expect(result.disabled).toHaveLength(0);
    expect(result.sorted).toHaveLength(0);
  });

  it("places all enabled tools in enabled group (enabled-only case)", () => {
    const tools = [makeTool("a"), makeTool("b"), makeTool("c")];
    const result = sortToolsForList(tools.map(hit), makeDaemon());
    expect(result.enabled).toHaveLength(3);
    expect(result.disabled).toHaveLength(0);
    expect(result.sorted).toHaveLength(3);
    expect(result.enabled.every((e) => e.enabled)).toBe(true);
    expect(result.enabled.every((e) => e.reason === null)).toBe(true);
  });

  it("places all disabled tools in disabled group (disabled-only case)", () => {
    const daemon = makeDaemon();
    const tools = [makeTool("x", () => "No session"), makeTool("y", () => "No session")];
    const result = sortToolsForList(tools.map(hit), daemon);
    expect(result.enabled).toHaveLength(0);
    expect(result.disabled).toHaveLength(2);
    expect(result.disabled.every((e) => !e.enabled)).toBe(true);
    expect(result.disabled.every((e) => e.reason === "No session")).toBe(true);
  });

  it("separates enabled and disabled correctly in mixed case", () => {
    const daemon = makeDaemon();
    const tools = [
      makeTool("e1"),
      makeTool("d1", () => "reason A"),
      makeTool("e2"),
      makeTool("d2", () => "reason B"),
    ];
    const result = sortToolsForList(tools.map(hit), daemon);
    expect(result.enabled.map((e) => e.tool.id)).toEqual(["e1", "e2"]);
    expect(result.disabled.map((e) => e.tool.id)).toEqual(["d1", "d2"]);
  });

  it("preserves registry order within enabled group", () => {
    const daemon = makeDaemon();
    // fuzzyRanked order: b, c, a (e.g. after fuzzy ranking)
    const b = makeTool("b");
    const c = makeTool("c");
    const a = makeTool("a");
    const result = sortToolsForList([hit(b), hit(c), hit(a)], daemon);
    expect(result.enabled.map((e) => e.tool.id)).toEqual(["b", "c", "a"]);
  });

  it("preserves registry order within disabled group", () => {
    const daemon = makeDaemon();
    const x = makeTool("x", () => "no");
    const z = makeTool("z", () => "no");
    const m = makeTool("m", () => "no");
    const result = sortToolsForList([hit(x), hit(z), hit(m)], daemon);
    expect(result.disabled.map((e) => e.tool.id)).toEqual(["x", "z", "m"]);
  });

  it("assigns logicalIndex as contiguous 0-based: enabled first then disabled", () => {
    const daemon = makeDaemon();
    const e1 = makeTool("e1");
    const d1 = makeTool("d1", () => "reason");
    const e2 = makeTool("e2");
    const result = sortToolsForList([hit(e1), hit(d1), hit(e2)], daemon);
    // enabled: e1 → 0, e2 → 1; disabled: d1 → 2
    expect(result.enabled.map((e) => e.logicalIndex)).toEqual([0, 1]);
    expect(result.disabled.map((e) => e.logicalIndex)).toEqual([2]);
    expect(result.sorted.map((e) => e.logicalIndex)).toEqual([0, 1, 2]);
  });

  it("calls disabledReason exactly once per ToolDef (FR-003 / ADR-0047)", () => {
    const daemon = makeDaemon();
    const spyA = vi.fn(() => null);
    const spyB = vi.fn(() => "reason");
    const a = makeTool("a", spyA);
    const b = makeTool("b", spyB);
    sortToolsForList([hit(a), hit(b)], daemon);
    expect(spyA).toHaveBeenCalledTimes(1);
    expect(spyB).toHaveBeenCalledTimes(1);
  });

  it("sorted is enabled+disabled concatenated in logicalIndex order", () => {
    const daemon = makeDaemon();
    const e = makeTool("e");
    const d = makeTool("d", () => "reason");
    const result = sortToolsForList([hit(e), hit(d)], daemon);
    expect(result.sorted.map((e) => e.tool.id)).toEqual(["e", "d"]);
    expect(result.sorted.at(0)?.enabled).toBe(true);
    expect(result.sorted.at(1)?.enabled).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// resolveCursorBySelectedToolId
// ---------------------------------------------------------------------------

// Helper: build a minimal SortedToolEntry array directly.
function makeEntry(id: string, logicalIndex: number, enabled: boolean): SortedToolEntry {
  return {
    tool: makeTool(id),
    enabled,
    reason: enabled ? null : "reason",
    logicalIndex,
  };
}

describe("resolveCursorBySelectedToolId", () => {
  it("returns -1 when sorted has no enabled entries", () => {
    const sorted = [makeEntry("d1", 0, false), makeEntry("d2", 1, false)];
    expect(resolveCursorBySelectedToolId("d1", 0, sorted)).toBe(-1);
    expect(resolveCursorBySelectedToolId(null, 0, sorted)).toBe(-1);
  });

  it("(a) returns same logical index when prevSelectedId remains enabled", () => {
    const sorted = [makeEntry("e1", 0, true), makeEntry("e2", 1, true), makeEntry("d1", 2, false)];
    expect(resolveCursorBySelectedToolId("e2", 1, sorted)).toBe(1);
  });

  it("(b) moves to nearest forward enabled when prevSelectedId is now disabled", () => {
    // sorted: e1(0,enabled), e2(1,disabled), e3(2,enabled)
    // prevSelectedId='e2' is disabled → not taken by rule (a).
    // FR-026 forward-first: from anchor=1, forward: index 1 disabled, index 2 → e3 enabled → return 2.
    const sorted = [
      makeEntry("e1", 0, true),
      makeEntry("e2", 1, false), // same id but now disabled
      makeEntry("e3", 2, true),
    ];
    expect(resolveCursorBySelectedToolId("e2", 1, sorted)).toBe(2);
  });

  it("(b) forward search finds nearest enabled (gone-id, forward wins)", () => {
    const sorted = [
      makeEntry("d1", 0, false),
      makeEntry("d2", 1, false), // prevSelectedId gone
      makeEntry("e1", 2, true),
    ];
    // prevLogicalIndex=1 (where gone tool was).
    // FR-026 forward-first: index 1 disabled, index 2 → e1 enabled → return 2.
    expect(resolveCursorBySelectedToolId("gone-id", 1, sorted)).toBe(2);
  });

  it("(c) same-id gone + no forward enabled → backward fallback finds nearest", () => {
    const sorted = [makeEntry("e1", 0, true), makeEntry("e2", 1, true), makeEntry("d1", 2, false)];
    // "gone-id" was at index 2. FR-026 forward-first: index 2 disabled, no more forward.
    // backward fallback: index 1 → e2 enabled → return 1.
    expect(resolveCursorBySelectedToolId("gone-id", 2, sorted)).toBe(1);
  });

  it("(e) prevSelectedId null + enabled entries → returns 0", () => {
    const sorted = [makeEntry("e1", 0, true), makeEntry("e2", 1, true)];
    expect(resolveCursorBySelectedToolId(null, 0, sorted)).toBe(0);
  });

  it("(e) prevSelectedId null + first entry disabled → returns first enabled index", () => {
    const sorted = [makeEntry("d1", 0, false), makeEntry("e1", 1, true)];
    // anchor=0 (null), backward: nothing. forward: 0 disabled, 1 enabled → 1.
    expect(resolveCursorBySelectedToolId(null, 0, sorted)).toBe(1);
  });

  it("prevSelectedId remains enabled at new index after reorder", () => {
    // Tool "e2" moved from index 1 to index 0 after a fuzzy re-rank.
    const sorted = [makeEntry("e2", 0, true), makeEntry("e1", 1, true)];
    expect(resolveCursorBySelectedToolId("e2", 1, sorted)).toBe(0);
  });
});
