import type { ServerFrame, SessionInfo } from "./server";
import type { ClientFrame } from "./client";

export function parseServerFrame(raw: string): ServerFrame | null {
  let v: unknown;
  try {
    v = JSON.parse(raw);
  } catch {
    return null;
  }
  // asciicast v2 配列: ["o", number, string]
  if (Array.isArray(v)) {
    if (v.length === 3 && v[0] === "o" && typeof v[1] === "number" && typeof v[2] === "string") {
      return ["o", v[1], v[2]];
    }
    return null;
  }
  if (typeof v !== "object" || v === null) {
    return null;
  }
  const obj = v as Record<string, unknown>;
  const k = obj.k;
  switch (k) {
    case "c":
      if (typeof obj.code !== "string") return null;
      return {
        k: "c",
        code: obj.code,
        data: obj.data as string | string[] | undefined,
      };
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
