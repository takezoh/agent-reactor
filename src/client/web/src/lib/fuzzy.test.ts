import { describe, expect, it } from "vitest";
import { fuzzyRank } from "./fuzzy";

const id = (s: string) => s;

describe("fuzzyRank", () => {
  it("returns items in input order with score 0 and empty ranges when query is empty", () => {
    const items = ["foo", "bar", "baz"];
    const hits = fuzzyRank(items, "", id);
    expect(hits.map((h) => h.item)).toEqual(["foo", "bar", "baz"]);
    expect(hits.every((h) => h.score === 0)).toBe(true);
    expect(hits.every((h) => h.ranges.length === 0)).toBe(true);
  });

  it("scores exact substring matches highest and returns a single contiguous range", () => {
    const items = ["foobar", "fxoxoxbar"];
    const hits = fuzzyRank(items, "foo", id);
    expect(hits.map((h) => h.item)).toEqual(["foobar", "fxoxoxbar"]);
    expect(hits.map((h) => h.ranges)).toEqual([[[0, 3]], expect.any(Array)]);
    const scores = hits.map((h) => h.score);
    expect(scores[0]).toBeGreaterThan(scores[1] ?? Number.NEGATIVE_INFINITY);
  });

  it("ranks consecutive matches above non-consecutive ones for the same text", () => {
    const consecutive = fuzzyRank(["fooBar"], "foo", id);
    const scattered = fuzzyRank(["fooBar"], "fbr", id);
    expect(consecutive.length).toBe(1);
    expect(scattered.length).toBe(1);
    const s1 = consecutive.map((h) => h.score)[0] ?? 0;
    const s2 = scattered.map((h) => h.score)[0] ?? 0;
    expect(s1).toBeGreaterThan(s2);
  });

  it("produces multiple ranges for non-consecutive matches", () => {
    const hits = fuzzyRank(["fooBar"], "fbr", id);
    const ranges = hits.flatMap((h) => h.ranges);
    expect(ranges.length).toBeGreaterThan(1);
    const flat = ranges.flat();
    expect(flat).toContain(0); // f
    expect(flat).toContain(3); // B
    expect(flat).toContain(5); // r
  });

  it("matches case-insensitively while preserving original text indices", () => {
    const lower = fuzzyRank(["FOO"], "foo", id);
    expect(lower.map((h) => h.ranges)).toEqual([[[0, 3]]]);
    const upper = fuzzyRank(["Bar"], "BAR", id);
    expect(upper.map((h) => h.ranges)).toEqual([[[0, 3]]]);
  });

  it("drops items whose text is missing any query character", () => {
    const items = ["apple", "banana", "cherry"];
    const hits = fuzzyRank(items, "xyz", id);
    expect(hits).toEqual([]);
  });

  it("drops items where query chars cannot be matched in order", () => {
    // 'oof' chars are present in 'foo' but not in order (no 'f' after the o's).
    const hits = fuzzyRank(["foo"], "oof", id);
    expect(hits).toEqual([]);
  });

  it("keeps input order for equal scores (stable sort)", () => {
    const items = ["abc", "abc", "abc"];
    const hits = fuzzyRank(items, "a", id);
    expect(hits.length).toBe(3);
    const scores = hits.map((h) => h.score);
    expect(scores[0]).toBe(scores[1]);
    expect(scores[1]).toBe(scores[2]);
  });

  it("uses getText to project arbitrary item types", () => {
    type Tool = { id: string; label: string };
    const items: Tool[] = [
      { id: "a", label: "deploy" },
      { id: "b", label: "develop" },
    ];
    const hits = fuzzyRank(items, "dep", (t) => t.label);
    expect(hits.map((h) => h.item.id)).toEqual(["a", "b"]);
    expect(hits.map((h) => h.ranges)[0]).toEqual([[0, 3]]);
  });

  it("gives a leading-match bonus over mid-string match", () => {
    const items = ["xxfoo", "foo"];
    const hits = fuzzyRank(items, "foo", id);
    expect(hits.map((h) => h.item)).toEqual(["foo", "xxfoo"]);
  });
});
