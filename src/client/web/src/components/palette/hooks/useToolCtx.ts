// useToolCtx — assembles a ToolCtx from daemon snapshot + httpFactory.
//
// Extracted from CommandPalette.tsx so both CommandPalette (for ParamSelectPhase)
// and ToolSelectPhase can share the same construction logic.
//
// FR refs: FR-025

import { useMemo } from "react";
import { makeSessionsApi } from "../../../api/sessions";
import { isValidSessionsApi } from "../../../lib/sessionsApiGuard";
import type { DaemonSnapshot, ToolCtx } from "../../../lib/tools";
import { useDaemonStore } from "../../../store/daemon";
import { useNotificationsStore } from "../../../store/notifications";
import { usePaletteStore } from "../../../store/palette";
import type { ActiveContextSnapshot } from "../../../store/palette_active_context";

/**
 * Build a ToolCtx from the daemon snapshot and httpFactory.
 *
 * Returns null when the http factory produces an invalid SessionsApi shape.
 * frozenActiveContext is forwarded as-is to ctx (pass undefined when not frozen).
 */
export function useToolCtx(
  daemon: DaemonSnapshot,
  httpFactory: (() => ToolCtx["http"]) | undefined,
  frozenActiveContext: ActiveContextSnapshot | undefined,
): ToolCtx | null {
  return useMemo<ToolCtx | null>(() => {
    const http = httpFactory ? httpFactory() : makeSessionsApi();
    if (!isValidSessionsApi(http)) {
      console.error("[palette] httpFactory returned an invalid SessionsApi; ctx not built", {
        keys: http === null || typeof http !== "object" ? typeof http : Object.keys(http),
      });
      return null;
    }
    const paletteState = usePaletteStore.getState();
    return {
      http,
      daemon,
      daemonActions: {
        selectSession(id) {
          useDaemonStore.getState().selectSession(id);
        },
      },
      notify: {
        success(m) {
          useNotificationsStore.getState().add({ level: "info", message: m });
        },
        error(m) {
          useNotificationsStore.getState().add({ level: "error", message: m });
        },
        add(input) {
          useNotificationsStore.getState().add(input);
        },
      },
      store: {
        close: paletteState.close,
      },
      frozenActiveContext,
    };
  }, [daemon, httpFactory, frozenActiveContext]);
}
