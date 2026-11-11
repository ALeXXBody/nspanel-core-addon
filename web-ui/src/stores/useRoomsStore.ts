import { create } from "zustand";
import { devtools } from "zustand/middleware";

import { LightType, SliderType } from "@/types";
import {
  stompService,
  type LightCommandOptions,
} from "@/services/stompService";
import { useUIStore } from "./useUIStore";
import { useConfigStore } from "./useConfigStore";
import type { NSPanelRoomStatus } from "@/generated/src/proto/protobuf_nspanel";

interface RoomsState {
  rooms: Record<string, NSPanelRoomStatus>;
  globalRoom: NSPanelRoomStatus | null;
  isLoaded: boolean;
  updateRoom: (roomData: NSPanelRoomStatus) => void;
  setGlobalRoom: (roomData: NSPanelRoomStatus) => void;
  resetRooms: () => void;
  handleLightToggle: (lightType: LightType, roomId?: string) => void;
  //roomId is used when calling handleLightToggle from the roomSelector. Not from main page
  handleMainPageLightSlider: (
    value: number,
    sliderType: SliderType,
    lockLightType?: typeof LightType.CEILING | typeof LightType.TABLE,
  ) => void;
}

export const useRoomsStore = create<RoomsState>()(
  devtools(
    (set, get) => ({
      rooms: {},
      globalRoom: null,
      isLoaded: false,

      updateRoom: (roomData) => {
        set(
          (state) => ({
            rooms: {
              ...state.rooms,
              [String(roomData.id)]: roomData,
            },
            isLoaded: true,
          }),
          false,
          "updateRoom",
        );
      },

      setGlobalRoom: (roomData) =>
        set({ globalRoom: roomData, isLoaded: true }, false, "setGlobalRoom"),

      resetRooms: () =>
        set(
          {
            rooms: {},
            globalRoom: null,
            isLoaded: false,
          },
          false,
          "resetRooms",
        ),
      handleLightToggle: (lightType, roomId) => {
        const { config, currentRoomId } = useConfigStore.getState();
        const { rooms, globalRoom } = get();
        const mainPageMode = useUIStore.getState().mainPageMode;

        const activeData = roomId
          ? rooms[roomId]
          : mainPageMode === "allLights"
            ? globalRoom
            : currentRoomId !== null
              ? rooms[currentRoomId]
              : null;
        if (!activeData || !config?.defaultLightBrightess || !config?.nspanelId)
          return;

        const isCurrentlyOn =
          lightType === LightType.TABLE
            ? activeData.numTableLightsOn > 0
            : activeData.numCeilingLightsOn > 0;

        const otherLightTypeDimLevel =
          lightType === LightType.TABLE
            ? activeData.ceilingLightsDimLevel
            : activeData.tableLightsDimLevel;

        const brightness = isCurrentlyOn
          ? 0
          : otherLightTypeDimLevel
            ? otherLightTypeDimLevel
            : config.defaultLightBrightess;
        console.log(
          "sending light toggle",
          brightness,
          // activeData.ceilingLightsColorTemperatureValue,
          // activeData.tableLightsColorTemperatureValue,
          mainPageMode,
        );
        stompService.sendMainPageLightCommand(
          lightType,
          { brightness: brightness },
          roomId ? Number(roomId) : Number(currentRoomId),
          // {
          //   isGlobal: mainPageMode === "allLights",
          //   nspanelId: config?.nspanelId,
          //   roomId: roomId ? Number(roomId) : Number(currentRoomId),
          // },
        );
      },
      handleMainPageLightSlider: (value, sliderType, lockLightType) => {
        const { config, currentRoomId } = useConfigStore.getState();
        const { rooms, globalRoom } = get();
        const mainPageMode = useUIStore.getState().mainPageMode;

        const activeData =
          mainPageMode === "allLights"
            ? globalRoom
            : currentRoomId !== null
              ? rooms[currentRoomId]
              : null;
        if (!activeData || !config?.nspanelId) return;

        const isCeilingOn = activeData.numCeilingLightsOn > 0;
        const isTableOn = activeData.numTableLightsOn > 0;

        let lightType: LightType;
        if (lockLightType) {
          lightType = lockLightType;
        } else {
          const key = `${isCeilingOn}-${isTableOn}`;

          const typeMap: Record<string, LightType> = {
            ["true-false"]: LightType.CEILING,
            ["false-true"]: LightType.TABLE,
            ["true-true"]: LightType.ALL,
            ["false-false"]: LightType.ALL,
          };
          lightType = typeMap[key];
        }
        const options: LightCommandOptions = {};
        if (sliderType === SliderType.BRIGHTNESS) {
          options.brightness = value;
        }
        if (sliderType === SliderType.COLORTEMP) {
          options.colorTemp = value;
        }

        stompService.sendMainPageLightCommand(
          lightType,
          options,
          Number(currentRoomId),
        );
      },
    }),
    { name: "RoomsStore" },
  ),
);

//   {isGlobal: mainPageMode === "allLights",
//   nspanelId: config?.nspanelId,
//   roomId: Number(currentRoomId),
// }
