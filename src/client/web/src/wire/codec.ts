import type { ServerFrame, SessionInfo } from "./server";
import type { ClientFrame } from "./client";

export function parseServerFrame(raw: string): ServerFrame | null {
  let v: unknown;
  try {
    v = JSON.parse(raw);
  } catch {
    return null;
  }
  // asciicast v2 配列: [number, "o", string] — Go wire.go:18 と同順
  if (Array.isArray(v)) {
    if (v.length === 3 && typeof v[0] === "number" && v[1] === "o" && typeof v[2] === "string") {
      return [v[0], "o", v[2]];
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
    case "h":
      if (
        !Array.isArray(obj.sessions) ||
        !Array.isArray(obj.features) ||
        typeof obj.serverTime !== "number"
      ) {
        return null;
      }
      return {
        k: "h",
        sessions: obj.sessions as SessionInfo[],
        activeSessionID: (obj.activeSessionID as string | null | undefined) ?? null,
        features: obj.features as string[],
        serverTime: obj.serverTime,
      };
    case "v":
      if (!Array.isArray(obj.sessions)) return null;
      return {
        k: "v",
        sessions: obj.sessions as SessionInfo[],
        activeSessionID: (obj.activeSessionID as string | null | undefined) ?? null,
      };
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
    default:
      return null;
  }
}

export function serializeClientFrame(f: ClientFrame): string {
  return JSON.stringify(f);
}
