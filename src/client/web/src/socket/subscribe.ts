// Active subscription set. The socket layer re-sends these on reconnect.
// Single-active-session model in β: only one entry expected, but the API is
// generalized to a set so γ multi-frame is a non-breaking extension.

export type SubscriptionEntry = {
  sessionId: string;
  // last assigned reqId so retry.ts can correlate Resp frames
  reqId: string;
};

export class SubscriptionRegistry {
  private active = new Map<string, SubscriptionEntry>();

  set(entry: SubscriptionEntry): void {
    this.active.set(entry.sessionId, entry);
  }

  remove(sessionId: string): void {
    this.active.delete(sessionId);
  }

  list(): SubscriptionEntry[] {
    return Array.from(this.active.values());
  }

  clear(): void {
    this.active.clear();
  }

  size(): number {
    return this.active.size;
  }
}
