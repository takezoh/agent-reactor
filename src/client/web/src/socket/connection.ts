import { useDaemonStore } from "../store/daemon";
import type { ClientFrame } from "../wire/client";
import { parseServerFrame, serializeClientFrame } from "../wire/codec";
import type { ControlFrame, OutputFrame, RespErrFrame, RespOKFrame } from "../wire/server";
import { backoffDelay, exceededAttempts } from "./backoff";
import { type RetryDeps, subscribeWithRetry } from "./retry";
import { SubscriptionRegistry } from "./subscribe";

export type ConnectionConfig = {
  ticketEndpoint: string; // POST /api/ws-ticket
  wsUrl: (ticket: string) => string; // build ws://host/...?ticket=
  bearerToken: string;
  // factories injectable for tests
  wsFactory?: (url: string) => WebSocket;
  sleep?: (ms: number) => Promise<void>;
  fetchFn?: typeof fetch;
};

type Pending = {
  resolve: (resp: RespOKFrame | RespErrFrame) => void;
};

export class Connection {
  private cfg: ConnectionConfig;
  private ws: WebSocket | null = null;
  private registry = new SubscriptionRegistry();
  private pending = new Map<string, Pending>();
  private reconnectAttempt = 0;
  private closedByUser = false;
  private reconnecting = false;
  private reqIdCounter = 0;

  constructor(cfg: ConnectionConfig) {
    this.cfg = cfg;
  }

  async start(): Promise<void> {
    useDaemonStore.getState().setStatus("connecting");
    await this.connect();
  }

  close(): void {
    this.closedByUser = true;
    this.drainPending();
    this.ws?.close();
    this.ws = null;
    this.registry.clear();
    useDaemonStore.getState().setStatus("closed");
  }

  send(frame: ClientFrame): void {
    this.ws?.send(serializeClientFrame(frame));
  }

  async subscribe(sessionId: string): Promise<void> {
    const deps: RetryDeps = {
      send: (s) => this.ws?.send(s),
      awaitResponse: (reqId) =>
        new Promise<RespOKFrame | RespErrFrame>((resolve) => {
          this.pending.set(reqId, { resolve });
        }),
      newReqId: () => this.nextReqId(),
      sleep: this.cfg.sleep ?? ((ms) => new Promise((r) => setTimeout(r, ms))),
    };
    const outcome = await subscribeWithRetry(sessionId, deps);
    if (outcome.status === "confirmed") {
      this.registry.set({ sessionId, reqId: outcome.reqId });
    }
  }

  async unsubscribe(sessionId: string): Promise<void> {
    const reqId = this.nextReqId();
    this.send({ k: "u", reqId, sessionId });
    this.registry.remove(sessionId);
  }

  private nextReqId(): string {
    this.reqIdCounter += 1;
    return `r${this.reqIdCounter}`;
  }

  private async connect(): Promise<void> {
    const fetchFn = this.cfg.fetchFn ?? fetch;
    const resp = await fetchFn(this.cfg.ticketEndpoint, {
      method: "POST",
      headers: { Authorization: `Bearer ${this.cfg.bearerToken}` },
    });
    if (!resp.ok) {
      throw new Error(`ws-ticket failed: ${resp.status}`);
    }
    const body = (await resp.json()) as { ticket: string };
    const wsFactory = this.cfg.wsFactory ?? ((u) => new WebSocket(u));
    this.ws = wsFactory(this.cfg.wsUrl(body.ticket));
    this.ws.onopen = () => this.handleOpen();
    this.ws.onmessage = (ev) => this.handleMessage(String(ev.data));
    this.ws.onclose = () => this.handleClose();
    // onerror is intentionally a noop: browsers always fire onclose after onerror,
    // so letting onerror also call handleClose would trigger reconnect twice.
    this.ws.onerror = () => {};
  }

  private handleOpen(): void {
    this.reconnectAttempt = 0;
    useDaemonStore.getState().setStatus("open");
    // resubscribe active sessions
    for (const entry of this.registry.list()) {
      void this.subscribe(entry.sessionId);
    }
  }

  private handleMessage(raw: string): void {
    const frame = parseServerFrame(raw);
    if (!frame) return;
    if (Array.isArray(frame)) {
      // OutputFrame — direct callback per FR-β07 (kHz output, UI must not block)
      this.onOutput?.(frame as OutputFrame);
      return;
    }
    switch (frame.k) {
      case "h":
        useDaemonStore.getState().seedHello(frame);
        break;
      case "v":
        useDaemonStore.getState().applyViewUpdate(frame);
        break;
      case "c":
        this.handleControl(frame);
        break;
      case "r":
      case "e": {
        const p = this.pending.get(frame.reqId);
        if (p) {
          p.resolve(frame);
          this.pending.delete(frame.reqId);
        }
        break;
      }
    }
  }

  private handleControl(frame: ControlFrame): void {
    // ControlFrame: code is int (omitted when 0), data carries event payload string
    if (frame.data === "daemon-disconnected") {
      useDaemonStore.getState().setDaemonDisconnected(true);
    }
  }

  private drainPending(): void {
    // Resolve all in-flight pending promises with a synthetic non-retryable error so
    // that awaiters (subscribeWithRetry) return immediately instead of hanging forever.
    for (const [reqId, p] of this.pending) {
      p.resolve({ k: "e", reqId, code: "connection-closed", message: "WebSocket closed" });
    }
    this.pending.clear();
  }

  private handleClose(): void {
    if (this.closedByUser) return;
    // Guard: onerror + onclose both fire in real browsers. Only run once.
    if (this.reconnecting) return;
    this.reconnecting = true;
    this.drainPending();
    useDaemonStore.getState().setStatus("reconnecting");
    if (exceededAttempts(this.reconnectAttempt)) {
      useDaemonStore.getState().setStatus("closed");
      this.reconnecting = false;
      return;
    }
    const delay = backoffDelay(this.reconnectAttempt);
    this.reconnectAttempt += 1;
    const sleep = this.cfg.sleep ?? ((ms) => new Promise((r) => setTimeout(r, ms)));
    void sleep(delay).then(() => {
      this.reconnecting = false;
      if (!this.closedByUser) {
        this.connect().catch(() => {
          this.handleClose();
        });
      }
    });
  }

  // hook for TerminalPane: called on output frames (FR-β07: kHz output, not via store)
  onOutput?: (frame: OutputFrame) => void;
}
