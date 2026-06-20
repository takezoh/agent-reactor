import { useDaemonStore } from "../store/daemon";

export function StatusBanner() {
  const status = useDaemonStore((s) => s.status);
  const daemonDisconnected = useDaemonStore((s) => s.daemonDisconnected);
  const show = status === "reconnecting" || status === "closed" || daemonDisconnected;
  if (!show) return null;
  const message =
    status === "closed"
      ? "connection closed"
      : daemonDisconnected
        ? "daemon disconnected, reconnecting…"
        : "reconnecting to server…";
  return <output className="status-banner">{message}</output>;
}
