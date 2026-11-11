import { create } from "zustand";
import { devtools, persist } from "zustand/middleware";
import { generateFriendlyName, generateRandomMac } from "@/utils/utils";
import type { NSPanelConfig } from "@/generated/src/proto/protobuf_nspanel";

interface ConfigState {
  virtualMac: string;
  friendlyName: string;
  config: NSPanelConfig | null;
  isLoaded: boolean;
  roomOrder: string[];
  currentRoomId: string | null;
  roomChangeDirection: "DIRECT" | "PREV" | "NEXT";
  resetDefaultRoom: boolean;

  initialize: () => void;
  setConfig: (newConfig: NSPanelConfig) => void;
  setDefaultRoom: () => void;
  setCurrentRoom: (id: string) => void;
  nextRoom: () => void;
  prevRoom: () => void;
  resetConfig: () => void;
}

export const useConfigStore = create<ConfigState>()(
  devtools(
    persist(
      (set, get) => ({
        virtualMac: "",
        friendlyName: "",
        config: null,
        isLoaded: false,
        roomOrder: [],
        currentRoomId: null,
        roomChangeDirection: "DIRECT",
        resetDefaultRoom: false,

        initialize: () => {
          const state = get();

          if (!state.virtualMac) {
            const mac = generateRandomMac();

            set({
              virtualMac: mac,
              friendlyName: generateFriendlyName(mac),
            });
          }
        },

        setConfig: (newConfig) => {
          // Extract the room order from the config
          const order =
            newConfig.roomInfos?.map((room) => String(room.roomId)) || [];

          set(
            {
              config: newConfig,
              isLoaded: true,
              roomOrder: order,
              currentRoomId: String(newConfig.defaultRoom) || order[0] || null,
            },
            false,
            "setConfig",
          );
        },
        setDefaultRoom: () =>
          set(
            {
              currentRoomId: String(get().config?.defaultRoom),
            },
            false,
            "setDefaultRoom",
          ),
        setCurrentRoom: (id: string) => {
          set({ currentRoomId: id }, false, "setCurrentRoom");
        },
        nextRoom: () => {
          const roomOrder = get().roomOrder;
          const currentRoom = get().currentRoomId;
          if (currentRoom) {
            const currentRoomIndex = roomOrder.indexOf(currentRoom);
            const nextRoomIndex = (currentRoomIndex + 1) % roomOrder.length;
            const nextRoom = roomOrder[nextRoomIndex];
            set({ currentRoomId: nextRoom }, false, "nextRoom");
            set({ roomChangeDirection: "NEXT" }, false, "roomChangeDirection");
          }
        },

        prevRoom: () => {
          const roomOrder = get().roomOrder;
          const currentRoom = get().currentRoomId;
          if (currentRoom) {
            const currentRoomIndex = roomOrder.indexOf(currentRoom);
            const prevRoomIndex =
              (currentRoomIndex - 1 + roomOrder.length) % roomOrder.length;
            const prevRoom = roomOrder[prevRoomIndex];
            set({ currentRoomId: prevRoom }, false, "prevRoom");
            set({ roomChangeDirection: "PREV" }, false, "roomChangeDirection");
          }
        },

        resetConfig: () =>
          set(
            {
              config: null,
              isLoaded: false,
              roomOrder: [],
              currentRoomId: null,
            },
            false,
            "resetConfig",
          ),
      }),
      {
        name: "panel-config-storage",
        partialize: (state) => ({
          virtualMac: state.virtualMac,
          friendlyName: state.friendlyName,
        }),
      },
    ),
    { name: "ConfigStore" },
  ),
);
useConfigStore.getState().initialize();
