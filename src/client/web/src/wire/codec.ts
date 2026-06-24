import type { ClientFrame } from "./client";
import type { ServerFrame, SessionInfo } from "./server";

// parseSessionInfoLoose validates that an object has at minimum the fields
// required for a valid SessionInfo wire value: id string, view object with card object.
function parseSessionInfoLoose(obj: unknown): obj is SessionInfo {
  if (typeof obj !== "object" || obj === null) return false;
  const sess = obj as Record<string, unknown>;
  if (typeof sess.id !== "string") return false;
  if (typeof sess.view !== "object" || sess.view === null) return false;
  const view = sess.view as Record<string, unknown>;
  if (typeof view.card !== "object" || view.card === null) return false;
  return true;
}

export function parseServerFrame(raw: string): ServerFrame | null {
  let v: unknown;
  try {
    v = JSON.parse(raw);
  } catch {
    return null;
  }
  // asciicast v2 配列 + sessionId: [number, "o", string, string] — Go wire.go と同順
  if (Array.isArray(v)) {
    if (
      v.length === 4 &&
      typeof v[0] === "number" &&
      v[1] === "o" &&
      typeof v[2] === "string" &&
      typeof v[3] === "string"
    ) {
      return [v[0], "o", v[2], v[3]];
    }
    return null;
  }
  if (typeof v !== "object" || v === null) {
    return null;
  }
  const obj = v as Record<string, unknown>;
  const k = obj.k;
  switch (k) {
    case "c": {
      // Go: code is int omitempty (absent when 0), data is string omitempty.
      if (obj.code !== undefined && typeof obj.code !== "number") return null;
      // Shallow validate data: must be string or absent (Go only emits string).
      if (obj.data !== undefined && typeof obj.data !== "string") return null;
      return {
        k: "c" as const,
        ...(typeof obj.code === "number" ? { code: obj.code } : {}),
        ...(typeof obj.data === "string" ? { data: obj.data } : {}),
      };
    }
    case "h": {
      if (
        !Array.isArray(obj.sessions) ||
        !Array.isArray(obj.features) ||
        typeof obj.serverTime !== "number"
      ) {
        return null;
      }
      if (!obj.sessions.every(parseSessionInfoLoose)) return null;
      const hFrame: import("./server").HelloFrame = {
        k: "h",
        sessions: obj.sessions as SessionInfo[],
        activeSessionID: (obj.activeSessionID as string | null | undefined) ?? null,
        features: obj.features as string[],
        serverTime: obj.serverTime,
      };
      return hFrame;
    }
    case "v": {
      if (!Array.isArray(obj.sessions)) return null;
      if (!obj.sessions.every(parseSessionInfoLoose)) return null;
      const vFrame: import("./server").ViewUpdateFrame = {
        k: "v",
        sessions: obj.sessions as SessionInfo[],
      };
      // Preserve undefined when the wire omits activeSessionID (Go omitempty
      // strips empty strings). The store's applyViewUpdate distinguishes
      // undefined ("no change, keep current selection") from null/string
      // ("override"). Coercing undefined→null here would clobber the user's
      // selection on every daemon broadcast because the daemon does not
      // track per-web-client selection — its EvtSessionsChanged carries
      // ActiveSessionID="" for web-only deployments.
      if (obj.activeSessionID !== undefined) {
        vFrame.activeSessionID = obj.activeSessionID as string | null;
      }
      return vFrame;
    }
    case "r":
      if (typeof obj.reqId !== "string") return null;
      return { k: "r", reqId: obj.reqId, body: obj.body };
    case "e":
      if (
        typeof obj.reqId !== "string" ||
        typeof obj.code !== "string" ||
        typeof obj.message !== "string"
      ) {
        return null;
      }
      return { k: "e", reqId: obj.reqId, code: obj.code, message: obj.message };
    case "tt": {
      if (typeof obj.sessionId !== "string" || typeof obj.line !== "string") return null;
      return { k: "tt" as const, sessionId: obj.sessionId, line: obj.line };
    }
    case "et": {
      if (typeof obj.sessionId !== "string" || typeof obj.line !== "string") return null;
      return { k: "et" as const, sessionId: obj.sessionId, line: obj.line };
    }
    case "n": {
      if (
        typeof obj.sessionId !== "string" ||
        typeof obj.cmd !== "number" ||
        typeof obj.nowMs !== "number"
      ) {
        return null;
      }
      if (obj.title !== undefined && typeof obj.title !== "string") return null;
      if (obj.body !== undefined && typeof obj.body !== "string") return null;
      return {
        k: "n" as const,
        sessionId: obj.sessionId,
        cmd: obj.cmd,
        ...(typeof obj.title === "string" ? { title: obj.title } : {}),
        ...(typeof obj.body === "string" ? { body: obj.body } : {}),
        nowMs: obj.nowMs,
      };
    }
    default:
      return null;
  }
}

export function serializeClientFrame(f: ClientFrame): string {
  return JSON.stringify(f);
}
