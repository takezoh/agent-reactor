// Thin HTTP wrapper around the daemon-backed /api/sessions endpoints used by
// the command palette and tool registry. This is the single seam through
// which web UI talks to the gateway for session CRUD + push.
//
// Wire formats mirror src/server/web/mux.go:
//   - POST   /api/sessions             → 201 Created, body: apiSessionInfo
//   - DELETE /api/sessions/{id}        → 204 No Content
//   - POST   /api/sessions/{id}/push   → 200 OK (empty body); 4xx/5xx error
//                                       envelope via gatewayError
//   - GET    /api/session-config       → 200 OK, body: apiSessionConfig
//
// The push route (POST /api/sessions/{id}/push) is implemented by
// src/server/web/mux_push.go (ADR-0045 SendCommand pattern; ADR-0046 409
// active-mismatch). pushCommand below is the client side of that contract.

import { readBearerTokenFromHash } from "../auth";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

/**
 * Error thrown for any non-2xx HTTP response. `status` is the HTTP status
 * code; `body` is the response body text (if any). The palette store branches
 * on `status === 401` to trigger an auth-error toast (FR-024).
 *
 * Network failures (fetch itself rejecting) surface as plain Error("network"),
 * NOT as ApiHttpError — those have no HTTP status to attach.
 */
export interface ApiHttpError extends Error {
  status: number;
  body?: string;
}

/**
 * Error thrown when an HTTP 2xx response carries an unparseable / unexpected
 * payload (JSON.parse failure, or successful parse but missing required
 * fields). Distinct from ApiHttpError because status is 200 — the server
 * "succeeded" from HTTP's point of view, but the wire-format contract is
 * broken. Distinct from the "network" sentinel because the network round-
 * trip itself worked.
 *
 * The palette store can branch on this to surface a "server returned an
 * unexpected response" toast rather than a generic crash; in the meantime
 * having a typed Error with a `cause` preserves the original SyntaxError /
 * shape diagnostic for Sentry / dev console.
 *
 * (Spec: review finding "minor — createSession / getSessionConfig res.json
 * leaking SyntaxError".)
 */
export interface ApiWireFormatError extends Error {
  wireFormat: true;
  url: string;
}

export interface CreateSessionPayload {
  project: string;
  command: string;
  worktree?: boolean;
  // ADR-0042: palette new-session payload wire mirror. "" / undefined / "auto"
  // all mean "follow project config"; "host" forces direct/host launch.
  // We expose only the "host" override on the type because palette UI never
  // sends explicit "auto" — omitting the field is the same thing on the wire.
  sandbox?: "host";
}

/**
 * Project entry returned by GET /api/session-config. session-config-extension
 * (a later task) will fill in real `isGit` / `isSandboxed` values; until then
 * the wire still returns plain `string[]` and we normalize to this shape with
 * both flags false (see makeSessionsApi.getSessionConfig).
 */
export interface SessionConfigProject {
  path: string;
  isGit: boolean;
  isSandboxed: boolean;
}

/**
 * SessionConfig is the normalized client view of the GET /api/session-config
 * response. We expose both legacy and forward-compat fields so palette code
 * doesn't have to change again when the server is upgraded:
 *
 *   - projectRoots / projectPaths: forward-compat (session-config-extension
 *     will add `project_roots` / `project_paths` to the wire). Until then,
 *     they're [] and the UI falls back to `projectPaths` only.
 *   - projects: normalized list of project entries (path + flags). Always
 *     populated, even when the server still returns the legacy `string[]`
 *     shape.
 *   - commands: curated [session].commands list.
 *   - pushCommands: forward-compat (session-config-extension will populate).
 *     Until then, []. (Normalized from the wire's `push_commands` field —
 *     keeping the client interface camelCase matches projectRoots etc.)
 */
export interface SessionConfig {
  projectRoots: string[];
  projectPaths: string[];
  projects: SessionConfigProject[];
  commands: string[];
  pushCommands: string[];
}

export interface SessionsApi {
  createSession(payload: CreateSessionPayload): Promise<{ id: string }>;
  deleteSession(id: string): Promise<void>;
  pushCommand(sessionId: string, command: string): Promise<void>;
  getSessionConfig(): Promise<SessionConfig>;
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// makeAuthHeaderFn returns an authHeader function with its own "already
// notified" flag, so a stale token-missing warning from one SessionsApi
// instance doesn't suppress the warning on a freshly constructed one (this
// is also what lets each test see the warning independently).
function makeAuthHeaderFn(): (url: string) => Record<string, string> {
  let bearerMissingNotified = false;
  return (url: string): Record<string, string> => {
    const token = readBearerTokenFromHash();
    if (!token) {
      // Dev-time hint only. The server still returns 401 (which palette-store
      // surfaces as a toast), but without this one-shot warn a developer who
      // forgot to put `#token=...` in the URL sees only "401 unauthorized"
      // with no indication that the client never even attached an
      // Authorization header. Surface it once per SessionsApi instance so the
      // dev console has a breadcrumb without spamming on every poll.
      // (Spec: review finding "minor — token missing silent path".)
      if (!bearerMissingNotified) {
        bearerMissingNotified = true;
        // eslint-disable-next-line no-console -- diagnostic, see comment above
        console.warn(
          `[api/sessions] no bearer token in window.location.hash; sending ${url} without Authorization header`,
        );
      }
      return {};
    }
    return { Authorization: `Bearer ${token}` };
  };
}

function makeHttpError(status: number, body: string): ApiHttpError {
  const err = new Error(`HTTP ${status}`) as ApiHttpError;
  err.status = status;
  if (body) err.body = body;
  return err;
}

// makeWireFormatError attaches the URL and a `cause` so the original parse
// failure (SyntaxError from JSON.parse, or a shape-check Error) is preserved
// for logError(Sentry) / dev console, while giving callers a single typed
// shape to branch on.
function makeWireFormatError(url: string, message: string, cause: unknown): ApiWireFormatError {
  // ES2022 Error `cause` (same constraint as makeNetworkError).
  const err = new Error(`${url}: ${message}`, { cause }) as ApiWireFormatError;
  err.wireFormat = true;
  err.url = url;
  return err;
}

// parseJsonBody wraps res.json() so a malformed-JSON failure surfaces as
// ApiWireFormatError instead of leaking the raw SyntaxError. The successful
// branch returns Record<string, unknown> so callers can do typed field
// extraction without `as any`.
async function parseJsonBody(url: string, res: Response): Promise<Record<string, unknown>> {
  let body: unknown;
  try {
    body = await res.json();
  } catch (e) {
    throw makeWireFormatError(url, "response is not valid JSON", e);
  }
  if (body === null || typeof body !== "object" || Array.isArray(body)) {
    throw makeWireFormatError(
      url,
      `response top-level is not a JSON object (got ${body === null ? "null" : Array.isArray(body) ? "array" : typeof body})`,
      body,
    );
  }
  return body as Record<string, unknown>;
}

// makeNetworkError wraps the underlying fetch failure (TypeError / DOMException
// / TLS / CORS / etc.) into an Error whose message is the sentinel "network"
// (palette-store branches on `=== 'network'`) but whose `cause` preserves the
// original exception. Without `cause` the original stack and DOMException name
// disappear, so AbortError vs offline vs CORS vs TLS all collapse into "fetch
// failed". (Spec: review finding "major — fetch reject silent network".)
function makeNetworkError(cause: unknown): Error {
  // `Error` constructor with `{ cause }` is ES2022 standard, supported by all
  // browsers we ship to (Chromium / Firefox / Safari ≥ 15.4) and by jsdom in
  // the vitest environment.
  return new Error("network", { cause });
}

async function request(fetchImpl: typeof fetch, url: string, init: RequestInit): Promise<Response> {
  let res: Response;
  try {
    res = await fetchImpl(url, init);
  } catch (e) {
    // AbortError (user-initiated cancel) is functionally different from a real
    // network failure: callers usually want to ignore it (no toast, no retry).
    // Re-throw it untouched so caller code that listens for AbortError sees
    // the original DOMException with its stack intact.
    if (e instanceof DOMException && e.name === "AbortError") {
      throw e;
    }
    if (e instanceof Error && e.name === "AbortError") {
      throw e;
    }
    throw makeNetworkError(e);
  }
  if (!res.ok) {
    // Read the body for diagnostics. response.text() can itself throw if the
    // body is already consumed or the stream errors. We do NOT swallow that
    // silently — empty catch is forbidden by project rules. Instead we
    // record a sentinel that preserves the failure mode so 5xx and
    // 5xx+body-read-error are distinguishable in logs.
    // (Spec: review finding "major — res.text() empty catch".)
    let body = "";
    try {
      body = await res.text();
    } catch (e) {
      const reason = e instanceof Error ? e.name : String(e);
      body = `<unreadable: ${reason}>`;
      // eslint-disable-next-line no-console -- diagnostic, see comment above
      console.warn(
        `[api/sessions] failed to read error body for ${url} status=${res.status}: ${reason}`,
        e,
      );
    }
    throw makeHttpError(res.status, body);
  }
  return res;
}

// Distinguishes "field absent" (legitimate forward/backward compat) from
// "field present but wrong type" (wire bug we want to surface).
function isAbsent(raw: unknown): boolean {
  return raw === undefined || raw === null;
}

// Legacy server returns Projects as string[]. session-config-extension will
// change it to SessionConfigProject[]. Normalize both so palette code never
// has to branch on shape.
//
// We silently accept three shapes:
//   1. absent / null            → []  (legacy server with no projects field)
//   2. string[]                 → [{path, isGit:false, isSandboxed:false}]
//   3. SessionConfigProject[]   → pass-through (post session-config-extension)
//
// Any OTHER shape (object instead of array, mixed garbage entries, missing
// `path` on a per-entry basis) is a wire-format bug. We still return a safe
// value so the palette UI doesn't crash, but we emit a console.warn so the
// silent-failure-hunter review concern doesn't recur: a gateway bug that
// drops `projects` mid-flight will at minimum leave a breadcrumb in the dev
// console instead of looking like "no projects configured".
// (Spec: review finding "major — getSessionConfig silent fallback".)
function normalizeProjects(raw: unknown): SessionConfigProject[] {
  if (isAbsent(raw)) return [];
  if (!Array.isArray(raw)) {
    // eslint-disable-next-line no-console -- diagnostic, see header comment
    console.warn(
      "[api/sessions] /api/session-config: `projects` field present but not an array; treating as empty. raw=",
      raw,
    );
    return [];
  }
  return raw.map((p, i): SessionConfigProject => {
    if (typeof p === "string") {
      return { path: p, isGit: false, isSandboxed: false };
    }
    if (p && typeof p === "object") {
      const o = p as Record<string, unknown>;
      const path = typeof o.path === "string" ? o.path : "";
      if (path === "") {
        // eslint-disable-next-line no-console -- diagnostic
        console.warn(
          `[api/sessions] /api/session-config: projects[${i}] object missing string \`path\` field. entry=`,
          p,
        );
      }
      const isGit = typeof o.isGit === "boolean" ? o.isGit : false;
      const isSandboxed = typeof o.isSandboxed === "boolean" ? o.isSandboxed : false;
      return { path, isGit, isSandboxed };
    }
    // eslint-disable-next-line no-console -- diagnostic
    console.warn(
      `[api/sessions] /api/session-config: projects[${i}] is neither string nor object; substituting empty entry. raw=`,
      p,
    );
    return { path: "", isGit: false, isSandboxed: false };
  });
}

// Same silent-fallback distinction as normalizeProjects: absent → [] is
// legitimate (forward-compat for fields the current server doesn't emit),
// but a present-but-not-an-array value is a wire bug worth surfacing.
function normalizeStringList(raw: unknown, fieldName: string): string[] {
  if (isAbsent(raw)) return [];
  if (!Array.isArray(raw)) {
    // eslint-disable-next-line no-console -- diagnostic, see normalizeProjects
    console.warn(
      `[api/sessions] /api/session-config: \`${fieldName}\` present but not an array; treating as empty. raw=`,
      raw,
    );
    return [];
  }
  return raw.filter((v): v is string => typeof v === "string");
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

export function makeSessionsApi(fetchImpl?: typeof fetch): SessionsApi {
  const f = fetchImpl ?? fetch.bind(globalThis);
  const authHeader = makeAuthHeaderFn();

  return {
    async createSession(payload: CreateSessionPayload): Promise<{ id: string }> {
      const url = "/api/sessions";
      const res = await request(f, url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...authHeader(url),
        },
        body: JSON.stringify(payload),
      });
      // Server returns apiSessionInfo {id, project, command, created_at}. We
      // only need id. parseJsonBody converts SyntaxError → ApiWireFormatError
      // so the JSON-parse path is no longer a silent typed leak.
      const body = await parseJsonBody(url, res);
      if (typeof body.id !== "string" || body.id === "") {
        throw makeWireFormatError(url, "missing string `id` in response", body);
      }
      return { id: body.id };
    },

    async deleteSession(id: string): Promise<void> {
      const url = `/api/sessions/${encodeURIComponent(id)}`;
      await request(f, url, {
        method: "DELETE",
        headers: { ...authHeader(url) },
      });
    },

    async pushCommand(sessionId: string, command: string): Promise<void> {
      // POST /api/sessions/{id}/push — body {command}, server returns 200 OK
      // with an empty body on success. The request helper raises ApiHttpError
      // for any non-2xx so palette-store can branch on status (401 → auth
      // toast / close per FR-024; 409 → keep open + error per ADR-0046; other
      // 4xx/5xx → generic error toast).
      //
      // encodeURIComponent guards the URL path: the server's session-id
      // allowlist rejects anything outside [A-Za-z0-9_-], but if a UI bug
      // ever leaks a stray '/' we don't want it to silently retarget a
      // different path segment (same defense as deleteSession above).
      const url = `/api/sessions/${encodeURIComponent(sessionId)}/push`;
      await request(f, url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...authHeader(url),
        },
        body: JSON.stringify({ command }),
      });
      // No body to parse — server writes only WriteHeader(200) on success
      // (see sendPushDriver in src/server/web/mux_push.go). Returning void
      // is the contract palette-store / tools-registry-dynamic-push expect.
    },

    async getSessionConfig(): Promise<SessionConfig> {
      const url = "/api/session-config";
      const res = await request(f, url, {
        method: "GET",
        headers: { ...authHeader(url) },
      });
      // parseJsonBody surfaces SyntaxError / non-object responses as a typed
      // ApiWireFormatError so palette-store can branch on it (review finding
      // "minor — res.json SyntaxError leak").
      const raw = await parseJsonBody(url, res);
      // Forward-compat: session-config-extension will add project_roots /
      // project_paths / push_commands to the wire. Until then they're absent
      // and we default to [] silently. normalizeStringList / normalizeProjects
      // distinguish absent (silent) from present-but-malformed (warn) so a
      // gateway bug that drops a field leaves a breadcrumb in the dev console.
      const projectRoots = normalizeStringList(raw.project_roots, "project_roots");
      const projectPaths = normalizeStringList(raw.project_paths, "project_paths");
      const commands = normalizeStringList(raw.commands, "commands");
      const pushCommands = normalizeStringList(raw.push_commands, "push_commands");
      const projects = normalizeProjects(raw.projects);
      return {
        projectRoots,
        projectPaths,
        projects,
        commands,
        pushCommands,
      };
    },
  };
}
