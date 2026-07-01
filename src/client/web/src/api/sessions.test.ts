import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  type ApiHttpError,
  type ApiWireFormatError,
  type CreateSessionPayload,
  makeSessionsApi,
} from "./sessions";

// makeResponse builds a fetch-compatible Response. Body defaults to "" so
// callers don't have to think about JSON.parse failures on 204/etc.
function makeResponse(
  body: string,
  status: number,
  headers: Record<string, string> = {},
): Response {
  return new Response(body, { status, headers });
}

function jsonResponse(value: unknown, status: number): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// asApiHttpError narrows an unknown caught value to ApiHttpError. We use it
// instead of a bare cast so a regression (e.g. throwing plain Error) fails
// the assertion at the type guard rather than at a downstream property read.
function asApiHttpError(e: unknown): ApiHttpError {
  if (!(e instanceof Error) || typeof (e as ApiHttpError).status !== "number") {
    throw new Error(`expected ApiHttpError, got: ${String(e)}`);
  }
  return e as ApiHttpError;
}

// asApiWireFormatError narrows to ApiWireFormatError. The `wireFormat: true`
// discriminant lets palette-store branch on wire-shape failure separately
// from HTTP status errors and the "network" sentinel.
function asApiWireFormatError(e: unknown): ApiWireFormatError {
  if (!(e instanceof Error) || (e as ApiWireFormatError).wireFormat !== true) {
    throw new Error(`expected ApiWireFormatError, got: ${String(e)}`);
  }
  return e as ApiWireFormatError;
}

const originalHash = "";

beforeEach(() => {
  // Default: no bearer token. Individual tests override window.location.hash.
  window.location.hash = originalHash;
});

afterEach(() => {
  window.location.hash = originalHash;
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// createSession
// ---------------------------------------------------------------------------

describe("createSession", () => {
  it("TestCreateSession_PostsJsonWithBearerAndReturnsId", async () => {
    window.location.hash = "#token=tok-abc";
    const fetchFn = vi
      .fn()
      .mockResolvedValue(
        jsonResponse(
          { id: "sess-1", project: "/p", command: "claude", created_at: "2026-06-24T00:00:00Z" },
          201,
        ),
      );
    const api = makeSessionsApi(fetchFn);
    const payload: CreateSessionPayload = {
      project: "/home/me/repo",
      command: "claude",
      worktree: true,
      sandbox: "host",
    };

    const out = await api.createSession(payload);

    expect(out).toEqual({ id: "sess-1" });
    expect(fetchFn).toHaveBeenCalledTimes(1);
    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/sessions");
    expect(init.method).toBe("POST");
    const headers = init.headers as Record<string, string>;
    expect(headers["Content-Type"]).toBe("application/json");
    expect(headers.Authorization).toBe("Bearer tok-abc");
    expect(JSON.parse(init.body as string)).toEqual(payload);
  });

  it("TestCreateSession_4xx_ThrowsApiHttpErrorWithStatus", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("bad project", 400));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.createSession({ project: "", command: "claude" });
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(400);
      expect(err.body).toBe("bad project");
    }
  });

  it("TestCreateSession_401_ThrowsApiHttpError401_ForPaletteAuthBranch", async () => {
    // FR-024: palette-store branches on status === 401 to surface an auth toast.
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("unauthorized", 401));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.createSession({ project: "/p", command: "claude" });
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(401);
    }
  });

  it("TestCreateSession_5xx_ThrowsApiHttpErrorWith5xxStatus", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("daemon down", 503));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.createSession({ project: "/p", command: "claude" });
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(503);
      expect(err.body).toBe("daemon down");
    }
  });

  it("TestCreateSession_FetchRejects_ThrowsGenericNetworkError", async () => {
    const fetchFn = vi.fn().mockRejectedValue(new TypeError("fetch failed"));
    const api = makeSessionsApi(fetchFn);

    await expect(api.createSession({ project: "/p", command: "claude" })).rejects.toThrow(
      "network",
    );
  });

  it("TestCreateSession_FetchRejects_PreservesCauseForLogging", async () => {
    // Review finding "major — fetch reject silent network": the original
    // TypeError / DOMException / CORS reason must reach logError(Sentry) via
    // `cause`. Otherwise we cannot tell offline from CORS from TLS in prod.
    const root = new TypeError("Failed to fetch: CORS preflight");
    const fetchFn = vi.fn().mockRejectedValue(root);
    const api = makeSessionsApi(fetchFn);

    try {
      await api.createSession({ project: "/p", command: "claude" });
      throw new Error("expected throw");
    } catch (e) {
      expect(e).toBeInstanceOf(Error);
      expect((e as Error).message).toBe("network");
      // `cause` preserves the original failure so the caller (and Sentry)
      // can distinguish failure modes that all surface as the same sentinel.
      expect((e as Error).cause).toBe(root);
    }
  });

  it("TestCreateSession_FetchAborted_RethrowsAbortErrorUntouched", async () => {
    // User-initiated cancel (AbortController.abort()) is functionally
    // different from a network failure. Callers usually want to ignore it
    // (no toast, no retry). We re-throw the original DOMException so its
    // `.name === 'AbortError'` shape survives.
    const aborted = new DOMException("aborted", "AbortError");
    const fetchFn = vi.fn().mockRejectedValue(aborted);
    const api = makeSessionsApi(fetchFn);

    try {
      await api.createSession({ project: "/p", command: "claude" });
      throw new Error("expected throw");
    } catch (e) {
      // Same identity as the thrown DOMException — not wrapped in a generic
      // "network" Error.
      expect(e).toBe(aborted);
      expect((e as Error).message).not.toBe("network");
    }
  });

  it("TestCreateSession_MalformedJson_ThrowsApiWireFormatErrorWithCause", async () => {
    // Review finding "minor — res.json SyntaxError leak": if the server
    // returns Content-Type: application/json but a malformed body, the raw
    // SyntaxError used to leak out untyped. It must surface as a typed
    // ApiWireFormatError with the original SyntaxError preserved as `cause`
    // so palette-store can branch separately from ApiHttpError and the
    // network sentinel.
    const fetchFn = vi.fn().mockResolvedValue(
      new Response("{not valid json", {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const api = makeSessionsApi(fetchFn);

    try {
      await api.createSession({ project: "/p", command: "claude" });
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiWireFormatError(e);
      expect(err.url).toBe("/api/sessions");
      // Underlying SyntaxError reachable for Sentry / dev console.
      expect(err.cause).toBeInstanceOf(SyntaxError);
    }
  });

  it("TestCreateSession_MissingId_ThrowsApiWireFormatErrorNotPlainError", async () => {
    // The previous "missing id in response" plain Error meant palette-store
    // couldn't distinguish a wire-shape bug from an arbitrary runtime
    // failure. Surface it as ApiWireFormatError instead.
    const fetchFn = vi
      .fn()
      .mockResolvedValue(jsonResponse({ project: "/p", command: "claude" }, 201));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.createSession({ project: "/p", command: "claude" });
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiWireFormatError(e);
      expect(err.message).toMatch(/missing string `id`/);
      expect(err.url).toBe("/api/sessions");
    }
  });

  it("TestCreateSession_NonObjectJson_ThrowsApiWireFormatError", async () => {
    // Top-level JSON array / string / number is a wire-format violation
    // (server contract is an object). Must throw ApiWireFormatError, not
    // crash at a downstream property access.
    const fetchFn = vi.fn().mockResolvedValue(
      new Response('"just a string"', {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const api = makeSessionsApi(fetchFn);

    try {
      await api.createSession({ project: "/p", command: "claude" });
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiWireFormatError(e);
      expect(err.message).toMatch(/top-level is not a JSON object/);
    }
  });

  it("TestCreateSession_NoBearer_OmitsAuthorizationHeaderAndEmitsWarn", async () => {
    // Review finding "minor — token missing silent path": when the developer
    // forgets `#token=...` in the URL, the request goes out without auth
    // (server returns 401, user sees toast), but the dev console should
    // also receive a one-shot warning so the cause is obvious in local
    // debugging. Warning is per-SessionsApi instance.
    window.location.hash = "";
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    // Each call needs its own Response — Response bodies can only be read
    // once, so a shared mockResolvedValue would throw on the second call.
    const fetchFn = vi
      .fn()
      .mockImplementation(() => Promise.resolve(jsonResponse({ id: "sess-2" }, 201)));
    const api = makeSessionsApi(fetchFn);

    await api.createSession({ project: "/p", command: "claude" });
    await api.createSession({ project: "/p", command: "claude" });

    // Auth header omitted on both requests.
    for (const call of fetchFn.mock.calls) {
      const [, init] = call as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers.Authorization).toBeUndefined();
    }
    // But the warn fired only once across both calls.
    const noBearerWarns = warnSpy.mock.calls.filter(
      (args) => typeof args[0] === "string" && args[0].includes("no bearer token"),
    );
    expect(noBearerWarns).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// deleteSession
// ---------------------------------------------------------------------------

describe("deleteSession", () => {
  it("TestDeleteSession_204_Returns", async () => {
    window.location.hash = "#token=tok-x";
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("", 204));
    const api = makeSessionsApi(fetchFn);

    await expect(api.deleteSession("sess-1")).resolves.toBeUndefined();

    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/sessions/sess-1");
    expect(init.method).toBe("DELETE");
    const headers = init.headers as Record<string, string>;
    expect(headers.Authorization).toBe("Bearer tok-x");
  });

  it("TestDeleteSession_EncodesId", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("", 204));
    const api = makeSessionsApi(fetchFn);

    // The server's session ID allowlist rejects "/" but encodeURIComponent
    // protects us if a UI bug ever passes through a stray character — verify
    // it's percent-encoded so the URL never lifts out into another path
    // segment.
    await api.deleteSession("a/b");
    const [url] = fetchFn.mock.calls[0] as [string];
    expect(url).toBe("/api/sessions/a%2Fb");
  });

  it("TestDeleteSession_4xx_ThrowsApiHttpError", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("not found", 404));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.deleteSession("sess-missing");
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(404);
    }
  });
});

// ---------------------------------------------------------------------------
// pushCommand
// ---------------------------------------------------------------------------

describe("pushCommand", () => {
  it("TestPushCommand_200_PostsJsonBodyWithBearerAndResolves", async () => {
    // Happy path: server returns 200 with empty body (see sendPushDriver in
    // src/server/web/mux_push.go). pushCommand resolves to void; URL is
    // /api/sessions/{id}/push; body is JSON {command}; Authorization +
    // Content-Type headers are present.
    window.location.hash = "#token=tok-push";
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("", 200));
    const api = makeSessionsApi(fetchFn);

    await expect(api.pushCommand("sess-1", "/clear")).resolves.toBeUndefined();

    expect(fetchFn).toHaveBeenCalledTimes(1);
    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/sessions/sess-1/push");
    expect(init.method).toBe("POST");
    const headers = init.headers as Record<string, string>;
    expect(headers["Content-Type"]).toBe("application/json");
    expect(headers.Authorization).toBe("Bearer tok-push");
    expect(JSON.parse(init.body as string)).toEqual({ command: "/clear" });
  });

  it("TestPushCommand_EncodesSessionId", async () => {
    // Defense-in-depth: the server's session-id allowlist rejects '/' so this
    // input would never land in production, but if a UI bug leaks a stray
    // separator we must percent-encode it instead of letting it lift out into
    // a different path segment (same guarantee as deleteSession).
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("", 200));
    const api = makeSessionsApi(fetchFn);

    await api.pushCommand("a/b", "/clear");
    const [url] = fetchFn.mock.calls[0] as [string];
    expect(url).toBe("/api/sessions/a%2Fb/push");
  });

  it("TestPushCommand_401_ThrowsApiHttpError401_ForPaletteAuthBranch", async () => {
    // FR-024: palette-store.submit branches on status === 401 to surface an
    // auth-error toast AND close the palette (token is stale, retry is
    // pointless). Must reach the catch as ApiHttpError with status=401.
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("unauthorized", 401));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.pushCommand("sess-1", "/clear");
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(401);
      expect(err.body).toBe("unauthorized");
    }
  });

  it("TestPushCommand_404_ThrowsApiHttpError404", async () => {
    // ADR-0045/0046: 404 = session id is unknown to the daemon. palette-store
    // surfaces it as a generic error and keeps the palette open so the user
    // can pick a different tool (the stale entry can then be re-evaluated by
    // disabledReason on the next snapshot).
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("session not found", 404));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.pushCommand("sess-gone", "/clear");
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(404);
      expect(err.body).toBe("session not found");
    }
  });

  it("TestPushCommand_409_ThrowsApiHttpError409_ForActiveMismatch", async () => {
    // FR-026 / ADR-0046: stale-tab guard. The path id is known but does not
    // match the daemon-global ActiveSessionID, so the server returns 409 to
    // refuse the silent retarget. palette-store catches and keeps the palette
    // open with an error message so disabledReason can re-evaluate against
    // the fresh snapshot.
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("path session id does not match", 409));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.pushCommand("sess-stale", "/clear");
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(409);
      expect(err.body).toMatch(/does not match/);
    }
  });

  it("TestPushCommand_413_ThrowsApiHttpError413", async () => {
    // Reviewer fix (major T4): the server rejects oversized push bodies
    // with 413 (body too large) — see src/server/web/mux_push.go's
    // MaxBytesReader guard. The client must surface this as a typed
    // ApiHttpError with status=413 so palette-store can keep the palette
    // open with an inline-error explaining the rejection (consistent with
    // the 4xx-non-401 branch) rather than crashing or showing a generic
    // toast.
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("request body too large", 413));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.pushCommand("sess-1", "/clear");
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(413);
      expect(err.body).toMatch(/too large/);
    }
  });

  it("TestPushCommand_5xx_ThrowsApiHttpErrorWith5xxStatus", async () => {
    // Daemon-down / RPC failure surfaces as 502/503/504 via handleProtoError
    // on the server. palette-store surfaces a generic "server error" toast.
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("daemon unavailable", 503));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.pushCommand("sess-1", "/clear");
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(503);
      expect(err.body).toBe("daemon unavailable");
    }
  });

  it("TestPushCommand_FetchRejects_ThrowsGenericNetworkError", async () => {
    // Fetch itself failing (offline, CORS preflight, TLS) surfaces as the
    // "network" sentinel Error, not as ApiHttpError. palette-store treats it
    // the same as a 5xx (generic error toast).
    const fetchFn = vi.fn().mockRejectedValue(new TypeError("fetch failed"));
    const api = makeSessionsApi(fetchFn);

    await expect(api.pushCommand("sess-1", "/clear")).rejects.toThrow("network");
  });

  it("TestPushCommand_FetchRejects_PreservesCauseForLogging", async () => {
    // Symmetry with createSession: the original TypeError / DOMException must
    // reach Sentry through `cause` so offline vs CORS vs TLS stay
    // distinguishable in prod logs.
    const root = new TypeError("Failed to fetch: TLS handshake");
    const fetchFn = vi.fn().mockRejectedValue(root);
    const api = makeSessionsApi(fetchFn);

    try {
      await api.pushCommand("sess-1", "/clear");
      throw new Error("expected throw");
    } catch (e) {
      expect(e).toBeInstanceOf(Error);
      expect((e as Error).message).toBe("network");
      expect((e as Error).cause).toBe(root);
    }
  });

  it("TestPushCommand_FetchAborted_RethrowsAbortErrorUntouched", async () => {
    // User cancel (palette closed mid-flight via AbortController) re-throws
    // the original DOMException so callers can ignore it cleanly.
    const aborted = new DOMException("aborted", "AbortError");
    const fetchFn = vi.fn().mockRejectedValue(aborted);
    const api = makeSessionsApi(fetchFn);

    try {
      await api.pushCommand("sess-1", "/clear");
      throw new Error("expected throw");
    } catch (e) {
      expect(e).toBe(aborted);
      expect((e as Error).message).not.toBe("network");
    }
  });

  it("TestPushCommand_NoBearer_OmitsAuthorizationHeader", async () => {
    // Same per-instance one-shot warn that createSession / deleteSession use
    // when window.location.hash has no #token=... — the dev console gets a
    // breadcrumb but the request still fires (server returns 401 → palette
    // store surfaces the auth toast).
    window.location.hash = "";
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("", 200));
    const api = makeSessionsApi(fetchFn);

    await api.pushCommand("sess-1", "/clear");

    const [, init] = fetchFn.mock.calls[0] as [string, RequestInit];
    const headers = init.headers as Record<string, string>;
    expect(headers.Authorization).toBeUndefined();
    // Content-Type still set; only Authorization is conditional.
    expect(headers["Content-Type"]).toBe("application/json");
    const noBearerWarns = warnSpy.mock.calls.filter(
      (args) => typeof args[0] === "string" && args[0].includes("no bearer token"),
    );
    expect(noBearerWarns).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// getSessionConfig
// ---------------------------------------------------------------------------

describe("getSessionConfig", () => {
  it("TestGetSessionConfig_LegacyProjectsStringArray_NormalizedToObjects", async () => {
    // Today's server (pre session-config-extension) returns projects as
    // plain string[]. The client normalizes to {path, isGit:false,
    // isSandboxed:false} so palette UI never branches on shape.
    window.location.hash = "#token=tok-z";
    const fetchFn = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          default_command: "claude",
          commands: ["claude", "codex"],
          projects: ["/home/me/repo", "/home/me/other"],
        },
        200,
      ),
    );
    const api = makeSessionsApi(fetchFn);

    const cfg = await api.getSessionConfig();

    expect(cfg.commands).toEqual(["claude", "codex"]);
    expect(cfg.projects).toEqual([
      { path: "/home/me/repo", isGit: false, isSandboxed: false },
      { path: "/home/me/other", isGit: false, isSandboxed: false },
    ]);
    // Forward-compat fields default to [] when absent on the wire.
    expect(cfg.projectRoots).toEqual([]);
    expect(cfg.projectPaths).toEqual([]);
    expect(cfg.pushCommands).toEqual([]);

    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/session-config");
    expect(init.method).toBe("GET");
    const headers = init.headers as Record<string, string>;
    expect(headers.Authorization).toBe("Bearer tok-z");
  });

  it("TestGetSessionConfig_NewServerWithObjectProjects_PassesThrough", async () => {
    // Post-extension wire: projects already a list of objects, plus
    // project_roots / project_paths / push_commands populated.
    const fetchFn = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          default_command: "claude",
          commands: ["claude"],
          project_roots: ["/home/me/work"],
          project_paths: ["/home/me/extra"],
          projects: [
            { path: "/home/me/work/a", isGit: true, isSandboxed: false },
            { path: "/home/me/extra", isGit: false, isSandboxed: true },
          ],
          push_commands: ["/clear", "/exit"],
          font_family: "HackGen Console NF",
          font_size: 16,
        },
        200,
      ),
    );
    const api = makeSessionsApi(fetchFn);

    const cfg = await api.getSessionConfig();

    expect(cfg.projectRoots).toEqual(["/home/me/work"]);
    expect(cfg.projectPaths).toEqual(["/home/me/extra"]);
    expect(cfg.projects).toEqual([
      { path: "/home/me/work/a", isGit: true, isSandboxed: false },
      { path: "/home/me/extra", isGit: false, isSandboxed: true },
    ]);
    expect(cfg.pushCommands).toEqual(["/clear", "/exit"]);
    // [terminal] font_family / font_size flow through to the client view.
    expect(cfg.fontFamily).toBe("HackGen Console NF");
    expect(cfg.fontSize).toBe(16);
  });

  it("TestGetSessionConfig_FontFieldsAbsent_DefaultToUnset", async () => {
    // Old gateway (or unset [terminal] font_* keys): font_family / font_size
    // are absent from the wire. The client must default to "" / 0 so
    // TerminalPane leaves the xterm.js built-in font untouched rather than
    // blanking the grid font.
    const fetchFn = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          default_command: "claude",
          commands: ["claude"],
          projects: [],
        },
        200,
      ),
    );
    const api = makeSessionsApi(fetchFn);

    const cfg = await api.getSessionConfig();

    expect(cfg.fontFamily).toBe("");
    expect(cfg.fontSize).toBe(0);
  });

  it("TestGetSessionConfig_401_ThrowsApiHttpError401", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("unauthorized", 401));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.getSessionConfig();
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(401);
    }
  });

  it("TestGetSessionConfig_500_ThrowsApiHttpError500", async () => {
    const fetchFn = vi.fn().mockResolvedValue(makeResponse("settings.toml broken", 500));
    const api = makeSessionsApi(fetchFn);

    try {
      await api.getSessionConfig();
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(500);
      expect(err.body).toBe("settings.toml broken");
    }
  });

  it("TestGetSessionConfig_FetchRejects_ThrowsGenericNetworkError", async () => {
    const fetchFn = vi.fn().mockRejectedValue(new TypeError("fetch failed"));
    const api = makeSessionsApi(fetchFn);

    await expect(api.getSessionConfig()).rejects.toThrow("network");
  });

  it("TestGetSessionConfig_BodyReadFails_RecordsUnreadableMarkerAndWarns", async () => {
    // Review finding "major — res.text() empty catch": when the body read
    // itself fails (already-consumed stream / network drop mid-body /
    // backpressure error), we must NOT swallow it silently. The body field
    // gets an `<unreadable: ...>` sentinel and a console.warn fires so the
    // dev can distinguish 5xx vs 5xx+body-read-error.
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const fakeResponse = {
      ok: false,
      status: 500,
      text: () => Promise.reject(new TypeError("stream drained")),
    } as unknown as Response;
    const fetchFn = vi.fn().mockResolvedValue(fakeResponse);
    const api = makeSessionsApi(fetchFn);

    try {
      await api.getSessionConfig();
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiHttpError(e);
      expect(err.status).toBe(500);
      expect(err.body).toMatch(/<unreadable:/);
    }
    const matched = warnSpy.mock.calls.filter(
      (args) => typeof args[0] === "string" && args[0].includes("failed to read error body"),
    );
    expect(matched.length).toBeGreaterThanOrEqual(1);
  });

  it("TestGetSessionConfig_MalformedProjectsObject_WarnsAndEmptyArray", async () => {
    // Review finding "major — silent normalize fallback": `projects` present
    // but not an array (e.g. proxy/gateway bug that wraps it) must produce
    // [] AND surface a console.warn so the breakage is traceable instead of
    // showing as "no projects configured".
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const fetchFn = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          commands: ["claude"],
          projects: { wrapped: ["/p"] }, // object instead of array
        },
        200,
      ),
    );
    const api = makeSessionsApi(fetchFn);

    const cfg = await api.getSessionConfig();

    expect(cfg.projects).toEqual([]);
    const matched = warnSpy.mock.calls.filter(
      (args) => typeof args[0] === "string" && args[0].includes("`projects` field present"),
    );
    expect(matched.length).toBeGreaterThanOrEqual(1);
  });

  it("TestGetSessionConfig_MalformedProjectsEntry_WarnsPerEntry", async () => {
    // Review finding "major — silent normalize fallback": per-entry garbage
    // (entry that is neither string nor object) and per-entry missing
    // `path` must each leave a console.warn breadcrumb instead of being
    // silently mapped to {path: ""}.
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const fetchFn = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          commands: ["claude"],
          projects: [
            "/good/string",
            { path: "/good/object", isGit: true, isSandboxed: false },
            { isGit: true }, // missing path
            42, // neither string nor object
          ],
        },
        200,
      ),
    );
    const api = makeSessionsApi(fetchFn);

    const cfg = await api.getSessionConfig();

    // Still returns a safe shape so palette UI doesn't crash.
    expect(cfg.projects).toEqual([
      { path: "/good/string", isGit: false, isSandboxed: false },
      { path: "/good/object", isGit: true, isSandboxed: false },
      { path: "", isGit: true, isSandboxed: false },
      { path: "", isGit: false, isSandboxed: false },
    ]);
    // Two diagnostics: one for the path-missing object, one for the non-
    // string-non-object entry.
    const pathWarns = warnSpy.mock.calls.filter(
      (args) => typeof args[0] === "string" && args[0].includes("missing string `path`"),
    );
    const shapeWarns = warnSpy.mock.calls.filter(
      (args) => typeof args[0] === "string" && args[0].includes("neither string nor object"),
    );
    expect(pathWarns).toHaveLength(1);
    expect(shapeWarns).toHaveLength(1);
  });

  it("TestGetSessionConfig_MalformedJson_ThrowsApiWireFormatErrorWithCause", async () => {
    // Same review finding ("minor — res.json SyntaxError leak") on the
    // getSessionConfig path: a daemon that returns garbled JSON must surface
    // as ApiWireFormatError, not as a raw SyntaxError that masquerades as a
    // generic runtime error.
    const fetchFn = vi.fn().mockResolvedValue(
      new Response("definitely not json", {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const api = makeSessionsApi(fetchFn);

    try {
      await api.getSessionConfig();
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiWireFormatError(e);
      expect(err.url).toBe("/api/session-config");
      expect(err.cause).toBeInstanceOf(SyntaxError);
    }
  });

  it("TestGetSessionConfig_TopLevelArray_ThrowsApiWireFormatError", async () => {
    // Wire-format contract: /api/session-config returns a JSON object. An
    // array (or other non-object) is a server bug — palette-store needs to
    // be able to branch on this without inheriting field-access crashes.
    const fetchFn = vi.fn().mockResolvedValue(
      new Response("[1,2,3]", {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const api = makeSessionsApi(fetchFn);

    try {
      await api.getSessionConfig();
      throw new Error("expected throw");
    } catch (e) {
      const err = asApiWireFormatError(e);
      expect(err.message).toMatch(/top-level is not a JSON object/);
    }
  });

  it("TestGetSessionConfig_MalformedStringList_WarnsAndEmptyArray", async () => {
    // Review finding "major — silent normalize fallback": absent string-list
    // fields are silent (legitimate forward-compat), but present-but-wrong-
    // type values must warn.
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const fetchFn = vi.fn().mockResolvedValue(
      jsonResponse(
        {
          commands: ["claude"],
          push_commands: "oops, a string", // wrong type
          // project_roots / project_paths absent → silent [] is OK.
        },
        200,
      ),
    );
    const api = makeSessionsApi(fetchFn);

    const cfg = await api.getSessionConfig();

    expect(cfg.pushCommands).toEqual([]);
    // No warn for legitimately-absent forward-compat fields.
    const absentWarns = warnSpy.mock.calls.filter(
      (args) =>
        typeof args[0] === "string" &&
        (args[0].includes("`project_roots`") || args[0].includes("`project_paths`")),
    );
    expect(absentWarns).toHaveLength(0);
    // But a warn for the present-but-wrong-type field.
    const wrongTypeWarns = warnSpy.mock.calls.filter(
      (args) => typeof args[0] === "string" && args[0].includes("`push_commands` present"),
    );
    expect(wrongTypeWarns).toHaveLength(1);
  });
});
