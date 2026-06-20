import { useState } from "react";
import type { Connection } from "../socket/connection";
import { useDaemonStore } from "../store/daemon";

export function CreateSessionForm({ conn }: { conn: Connection }) {
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const selectSession = useDaemonStore((s) => s.selectSession);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    setBusy(true);
    setErr(null);
    try {
      const resp = await fetch("/api/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title: title.trim() }),
      });
      if (!resp.ok) throw new Error(`POST /api/sessions failed: ${resp.status}`);
      const body = (await resp.json()) as { id: string };
      selectSession(body.id);
      await conn.subscribe(body.id);
      setTitle("");
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="create-session" onSubmit={onSubmit}>
      <input
        type="text"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        placeholder="New session title"
        disabled={busy}
      />
      <button type="submit" disabled={busy || !title.trim()}>
        Create
      </button>
      {err && <span className="error">{err}</span>}
    </form>
  );
}
