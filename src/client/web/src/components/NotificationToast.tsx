import { useEffect } from "react";
import type { Notification } from "../store/notifications";
import { useNotificationsStore } from "../store/notifications";

type ToastItemProps = {
  item: Notification;
  onDismiss: (id: number) => void;
};

function ToastItem({ item, onDismiss }: ToastItemProps): JSX.Element {
  useEffect(() => {
    const t = setTimeout(() => onDismiss(item.id), 5000);
    return () => clearTimeout(t);
  }, [item.id, onDismiss]);

  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: toast is supplemental UI; keyboard users rely on auto-dismiss
    <output
      aria-live="polite"
      onClick={() => onDismiss(item.id)}
      style={{
        display: "block",
        background: "#1e1e2e",
        color: "#cdd6f4",
        border: "1px solid #45475a",
        borderRadius: "6px",
        padding: "10px 14px",
        marginBottom: "8px",
        cursor: "pointer",
        minWidth: "240px",
        maxWidth: "360px",
        boxShadow: "0 2px 8px rgba(0,0,0,0.4)",
        wordBreak: "break-word",
      }}
    >
      {item.title != null && item.title !== "" ? (
        <>
          <div style={{ fontWeight: 600, marginBottom: "2px" }}>{item.title}</div>
          {item.body != null && item.body !== "" && (
            <div style={{ fontSize: "0.875em", opacity: 0.8 }}>{item.body}</div>
          )}
        </>
      ) : (
        <div>{item.message}</div>
      )}
    </output>
  );
}

export function NotificationToast(): JSX.Element {
  const items = useNotificationsStore((s) => s.items);
  const dismiss = useNotificationsStore((s) => s.dismiss);

  // Show only the latest 3 items
  const visible = items.slice(-3);

  return (
    <div
      className="notification-toast-stack"
      aria-label="notifications"
      style={{
        position: "fixed",
        top: "16px",
        right: "16px",
        zIndex: 9999,
        display: "flex",
        flexDirection: "column",
        alignItems: "flex-end",
      }}
    >
      {visible.map((item) => (
        <ToastItem key={item.id} item={item} onDismiss={dismiss} />
      ))}
    </div>
  );
}
