import { create } from "zustand";
import type { HelloFrame, SessionInfo, ViewUpdateFrame } from "../wire/server";

export type ConnectionStatus = "connecting" | "open" | "reconnecting" | "closed";

export type DaemonState = {
  sessions: SessionInfo[];
  activeSessionID: string | null;
  features: string[];
  serverTime: number;
  status: ConnectionStatus;
  // control frame で daemon-disconnected が来たかどうか。StatusBanner が参照。
  daemonDisconnected: boolean;

  // actions
  seedHello: (frame: HelloFrame) => void;
  applyViewUpdate: (frame: ViewUpdateFrame) => void;
  selectSession: (id: string | null) => void;
  setStatus: (status: ConnectionStatus) => void;
  setDaemonDisconnected: (v: boolean) => void;
  reset: () => void;
};

const initialState = {
  sessions: [] as SessionInfo[],
  activeSessionID: null as string | null,
  features: [] as string[],
  serverTime: 0,
  status: "connecting" as ConnectionStatus,
  daemonDisconnected: false,
};

export const useDaemonStore = create<DaemonState>()((set) => ({
  ...initialState,
  seedHello: (frame) =>
    set({
      sessions: frame.sessions,
      activeSessionID: frame.activeSessionID,
      features: frame.features,
      serverTime: frame.serverTime,
    }),
  applyViewUpdate: (frame) =>
    set((s) => {
      // best-effort identity preservation: keep the previous SessionInfo
      // object when its JSON shape is structurally unchanged. Cheap deep
      // compare via JSON.stringify is fine here (sessions[] is small —
      // 10s of entries — and the cost runs once per daemon push, ADR 0023).
      const byId = new Map(s.sessions.map((x) => [x.id, x]));
      const next = frame.sessions.map((incoming) => {
        const prev = byId.get(incoming.id);
        if (prev && JSON.stringify(prev) === JSON.stringify(incoming)) {
          return prev;
        }
        return incoming;
      });
      return {
        sessions: next,
        activeSessionID:
          frame.activeSessionID === undefined ? s.activeSessionID : frame.activeSessionID,
      };
    }),
  selectSession: (id) => set({ activeSessionID: id }),
  setStatus: (status) => set({ status }),
  setDaemonDisconnected: (v) => set({ daemonDisconnected: v }),
  reset: () => set(initialState),
}));
