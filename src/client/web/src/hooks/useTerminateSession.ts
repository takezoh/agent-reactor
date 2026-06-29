// useTerminateSession — 終了ボタン -> confirm dialog -> deleteSession の hook.
//
// 責務:
//   1. /api/sessions/{id} DELETE を呼ぶ
//   2. 204 / 404 (= already gone) は成功扱いで close
//   3. 5xx / network はエラー toast を出し dialog open のまま (pending=false)
//   4. 削除した session が active だった場合は selectNextActiveAfterDelete で
//      次セッションへ activeSessionID を切替える (ADR-0030 で view-update が
//      activeSessionID を載せないため、web 側で明示する必要がある)
//
// テスト容易性のため SessionsApi を optional 引数で差し替え可能.

import { useCallback, useMemo, useState } from "react";
import { type ApiHttpError, type SessionsApi, makeSessionsApi } from "../api/sessions";
import { selectNextActiveAfterDelete, useDaemonStore } from "../store/daemon";
import { useNotificationsStore } from "../store/notifications";

export interface UseTerminateSessionResult {
  /** Confirm 押下時に呼ぶ. true=dialog close OK / false=open のまま (エラー). */
  terminate: (id: string) => Promise<boolean>;
  /** API in-flight. confirm button の disabled / label 切替に使う. */
  pending: boolean;
}

function isHttpError(e: unknown): e is ApiHttpError {
  return e instanceof Error && typeof (e as ApiHttpError).status === "number";
}

function isNetworkError(e: unknown): boolean {
  return e instanceof Error && e.message === "network";
}

export function useTerminateSession(api?: SessionsApi): UseTerminateSessionResult {
  const [pending, setPending] = useState(false);
  // makeSessionsApi() は内部に bearerMissingNotified one-shot flag を持つ
  // (api/sessions.ts:128 のコメント参照). hook 寿命で 1 instance に固定する
  // ことで「token 欠落 warn が毎クリック出る」regression を防ぐ.
  const sessionsApi = useMemo(() => api ?? makeSessionsApi(), [api]);

  const terminate = useCallback(
    async (id: string): Promise<boolean> => {
      // 削除前 snapshot を pre-await で固定する. view-update は WS 経由で
      // 独立に到着するので await 後に getState() すると sessions から既に
      // deletedId が消えている race がある — その場合
      // selectNextActiveAfterDelete は first guard で null を返し、せっかく
      // 残っていた sibling sessions に遷移できず空白画面になる.
      const preSessions = useDaemonStore.getState().sessions;
      const preActiveId = useDaemonStore.getState().activeSessionID;
      setPending(true);
      let succeeded = false;
      try {
        await sessionsApi.deleteSession(id);
        succeeded = true;
      } catch (e) {
        if (isHttpError(e) && e.status === 404) {
          // 既に消えている — 望む状態なので成功扱い
          succeeded = true;
        } else if (isHttpError(e)) {
          useNotificationsStore.getState().add({
            level: "error",
            message: `セッション終了に失敗しました (HTTP ${e.status})`,
          });
        } else if (isNetworkError(e)) {
          useNotificationsStore.getState().add({
            level: "error",
            message: "セッション終了に失敗しました (ネットワークエラー)",
          });
        } else {
          useNotificationsStore.getState().add({
            level: "error",
            message: "セッション終了に失敗しました",
          });
        }
      }

      if (succeeded && preActiveId === id) {
        const next = selectNextActiveAfterDelete(preSessions, id);
        useDaemonStore.getState().selectSession(next);
      }

      setPending(false);
      return succeeded;
    },
    [sessionsApi],
  );

  return { terminate, pending };
}
