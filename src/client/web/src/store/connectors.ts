import { create } from "zustand";
import type { ConnectorInfo } from "../wire/server";

export type ConnectorsState = {
  connectors: ConnectorInfo[];
  setConnectors: (next: ConnectorInfo[]) => void;
  reset: () => void;
};

export const useConnectorsStore = create<ConnectorsState>()((set) => ({
  connectors: [],
  setConnectors: (next) => set({ connectors: next }),
  reset: () => set({ connectors: [] }),
}));
