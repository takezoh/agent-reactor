import { create } from "zustand";

export type TranscriptKind = "transcript" | "event-log";

export type TranscriptKey = `${string}:${TranscriptKind}`;
// Internal representation: `${sessionId}:${kind}`

const MAX_LINES_PER_BUFFER = 1000;

export type TranscriptBuffer = {
  lines: string[]; // Most recent N lines only (tail = newest)
  // Last byte offset received from REST backfill; incremented as WS tail arrives.
  restOffset: number;
};

export type TranscriptsState = {
  buffers: Record<TranscriptKey, TranscriptBuffer>;
  appendLine: (sessionId: string, kind: TranscriptKind, line: string) => void;
  appendBackfill: (
    sessionId: string,
    kind: TranscriptKind,
    lines: string[],
    newOffset: number,
  ) => void;
  clearSession: (sessionId: string) => void;
  reset: () => void;
};

export function bufferKey(sessionId: string, kind: TranscriptKind): TranscriptKey {
  return `${sessionId}:${kind}` as TranscriptKey;
}

export function selectBuffer(
  state: TranscriptsState,
  sessionId: string,
  kind: TranscriptKind,
): TranscriptBuffer | undefined {
  return state.buffers[bufferKey(sessionId, kind)];
}

export const useTranscriptStore = create<TranscriptsState>()((set) => ({
  buffers: {},

  appendLine: (sessionId, kind, line) =>
    set((s) => {
      const key = bufferKey(sessionId, kind);
      const existing = s.buffers[key];
      const lines = existing ? [...existing.lines, line] : [line];
      while (lines.length > MAX_LINES_PER_BUFFER) {
        lines.shift();
      }
      return {
        buffers: {
          ...s.buffers,
          [key]: {
            lines,
            restOffset: existing?.restOffset ?? 0,
          },
        },
      };
    }),

  appendBackfill: (sessionId, kind, lines, newOffset) =>
    set((s) => {
      const key = bufferKey(sessionId, kind);
      const existing = s.buffers[key];
      // REST backfill is historically OLDER than anything already in the
      // buffer: WebSocket tail frames may have arrived between the React
      // session-select and the REST response. Put the backfill in front,
      // not behind, so chronological order is preserved (and any duplicate
      // lines straddling the WS-REST boundary fall in temporal order
      // rather than appearing inverted to the user).
      const merged = existing ? [...lines, ...existing.lines] : [...lines];
      while (merged.length > MAX_LINES_PER_BUFFER) {
        merged.shift();
      }
      return {
        buffers: {
          ...s.buffers,
          [key]: {
            lines: merged,
            restOffset: newOffset,
          },
        },
      };
    }),

  clearSession: (sessionId) =>
    set((s) => {
      const next = { ...s.buffers };
      delete next[bufferKey(sessionId, "transcript")];
      delete next[bufferKey(sessionId, "event-log")];
      return { buffers: next };
    }),

  reset: () => set({ buffers: {} }),
}));
