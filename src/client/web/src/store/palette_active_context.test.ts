// UAC-001 / UAC-013 / FR-009 / FR-025 / FR-027 / FR-028
//
// Tests for pure helpers in palette_active_context.ts.
// No store state is touched; all tests are synchronous input → output.

import { describe, expect, it } from "vitest";

import type { SessionConfigProject } from "../api/sessions";
import type { SessionInfo } from "../wire/server";
import { deriveActiveContext, projBase, sid8 } from "./palette_active_context";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeProject(path: string): SessionConfigProject {
  return { path, isGit: false, isSandboxed: false };
}

function makeSession(id: string, project: string): SessionInfo {
  return {
    id,
    project,
    command: "arc",
    created_at: "2026-01-01T00:00:00Z",
    view: {
      card: {},
    },
  };
}

// ---------------------------------------------------------------------------
// projBase (FR-027)
// ---------------------------------------------------------------------------

describe("projBase", () => {
  it("returns basename for a normal Unix path with no collision", () => {
    const projects = [makeProject("/home/foo/bar")];
    expect(projBase("/home/foo/bar", projects)).toBe("bar");
  });

  it("returns input as-is when path ends with '/'", () => {
    const projects = [makeProject("/home/foo/bar/")];
    expect(projBase("/home/foo/bar/", projects)).toBe("/home/foo/bar/");
  });

  it("returns input as-is when path ends with '\\'", () => {
    const projects = [makeProject("C:\\work\\bar\\")];
    expect(projBase("C:\\work\\bar\\", projects)).toBe("C:\\work\\bar\\");
  });

  it("handles Windows paths with backslash separators", () => {
    const projects = [makeProject("C:\\work\\bar")];
    expect(projBase("C:\\work\\bar", projects)).toBe("bar");
  });

  it("returns '' for empty string", () => {
    expect(projBase("", [])).toBe("");
  });

  it("returns '' for whitespace-only string", () => {
    expect(projBase("   ", [])).toBe("   ");
  });

  it("adds disambiguator when two projects share the same basename", () => {
    const projects = [makeProject("/a/work"), makeProject("/b/work")];
    expect(projBase("/a/work", projects)).toBe("work (under a)");
    expect(projBase("/b/work", projects)).toBe("work (under b)");
  });

  it("adds disambiguator for Windows paths with same basename collision", () => {
    const projects = [makeProject("C:\\a\\work"), makeProject("C:\\b\\work")];
    expect(projBase("C:\\a\\work", projects)).toBe("work (under a)");
    expect(projBase("C:\\b\\work", projects)).toBe("work (under b)");
  });

  it("does not add disambiguator when only one project has a given basename", () => {
    const projects = [makeProject("/a/foo"), makeProject("/b/bar")];
    expect(projBase("/a/foo", projects)).toBe("foo");
    expect(projBase("/b/bar", projects)).toBe("bar");
  });

  it("handles path with no separator (root-level name)", () => {
    const projects = [makeProject("myproject")];
    expect(projBase("myproject", projects)).toBe("myproject");
  });

  it("mixed separator collision — Unix vs Windows path", () => {
    const projects = [makeProject("/a/work"), makeProject("C:\\b\\work")];
    expect(projBase("/a/work", projects)).toBe("work (under a)");
    expect(projBase("C:\\b\\work", projects)).toBe("work (under b)");
  });
});

// ---------------------------------------------------------------------------
// sid8 (FR-028)
// ---------------------------------------------------------------------------

describe("sid8", () => {
  it("returns first 8 chars of a long session ID", () => {
    expect(sid8("abcd1234efghijkl")).toBe("abcd1234");
  });

  it("returns entire string when shorter than 8 chars", () => {
    expect(sid8("abc")).toBe("abc");
  });

  it("returns exactly 8 chars when input is exactly 8 chars", () => {
    expect(sid8("12345678")).toBe("12345678");
  });

  it("returns empty string for empty input", () => {
    expect(sid8("")).toBe("");
  });
});

// ---------------------------------------------------------------------------
// deriveActiveContext (ADR-0058, FR-009, FR-025)
// ---------------------------------------------------------------------------

describe("deriveActiveContext", () => {
  it("returns kind=none when activeSessionID is null", () => {
    expect(deriveActiveContext(null, [], [])).toEqual({ kind: "none" });
  });

  it("returns kind=none when activeSessionID is empty string", () => {
    expect(deriveActiveContext("", [], [])).toEqual({ kind: "none" });
  });

  it("returns kind=unknown when session is not found in sessions list", () => {
    const result = deriveActiveContext("abcd1234efgh", [], []);
    expect(result).toEqual({
      kind: "unknown",
      sid8: "abcd1234",
      fullSessionId: "abcd1234efgh",
    });
  });

  it("returns kind=unknown when session.project is empty", () => {
    const sessions: SessionInfo[] = [makeSession("abcd1234efgh", "")];
    const result = deriveActiveContext("abcd1234efgh", sessions, []);
    expect(result).toEqual({
      kind: "unknown",
      sid8: "abcd1234",
      fullSessionId: "abcd1234efgh",
    });
  });

  it("returns kind=unknown when session project path is not in projects list", () => {
    const sessions: SessionInfo[] = [makeSession("abcd1234efgh", "/home/foo/bar")];
    const result = deriveActiveContext("abcd1234efgh", sessions, []);
    expect(result).toEqual({
      kind: "unknown",
      sid8: "abcd1234",
      fullSessionId: "abcd1234efgh",
    });
  });

  it("returns kind=resolved with correct fields when everything matches", () => {
    const sessions: SessionInfo[] = [makeSession("abcd1234efghijkl", "/home/foo/bar")];
    const projects: SessionConfigProject[] = [makeProject("/home/foo/bar")];
    const result = deriveActiveContext("abcd1234efghijkl", sessions, projects);
    expect(result).toEqual({
      kind: "resolved",
      projBase: "bar",
      sid8: "abcd1234",
      fullPath: "/home/foo/bar",
      fullSessionId: "abcd1234efghijkl",
    });
  });

  it("resolved: projBase includes disambiguator when multiple projects share basename", () => {
    const sessions: SessionInfo[] = [makeSession("abcd1234efghijkl", "/a/work")];
    const projects: SessionConfigProject[] = [makeProject("/a/work"), makeProject("/b/work")];
    const result = deriveActiveContext("abcd1234efghijkl", sessions, projects);
    expect(result).toEqual({
      kind: "resolved",
      projBase: "work (under a)",
      sid8: "abcd1234",
      fullPath: "/a/work",
      fullSessionId: "abcd1234efghijkl",
    });
  });
});
