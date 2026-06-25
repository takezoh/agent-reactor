// useDaemonSnapshot — subscribe to the 4 daemon primitives that feed
// selectDaemonSnapshot and return a memoised DaemonSnapshot.
//
// Extracted because four call sites (App.tsx, CommandPalette.tsx,
// ParamSelectPhase.tsx, ToolSelectPhase.tsx) were repeating the exact same
// primitive subscription + useMemo wiring. Centralising keeps the dep list
// in lockstep with selectDaemonSnapshot's input shape (one place to edit
// when a future field is added).
//
// We deliberately keep this in the store layer (not hooks/) so the file
// lives next to its sole producer (selectDaemonSnapshot in ./daemon.ts).

import { useMemo } from "react";
import type { DaemonSnapshot } from "../lib/tools";
import { selectDaemonSnapshot, useDaemonStore } from "./daemon";

export function useDaemonSnapshot(): DaemonSnapshot {
  const sessions = useDaemonStore((s) => s.sessions);
  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const activeOccupant = useDaemonStore((s) => s.activeOccupant);
  const sessionConfig = useDaemonStore((s) => s.sessionConfig);
  return useMemo<DaemonSnapshot>(
    () => selectDaemonSnapshot({ sessions, activeSessionID, activeOccupant, sessionConfig }),
    [sessions, activeSessionID, activeOccupant, sessionConfig],
  );
}
