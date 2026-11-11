import { create } from "zustand";
import { devtools } from "zustand/middleware";

type MainPageMode = "roomLights" | "allLights";
type Orientation = "landscape" | "portrait";

interface UIState {
  redirectTo: string | null;
  mainPageMode: MainPageMode;
  orientation: Orientation;
  isRoomSelectorOpen: boolean;
  setRedirectTo: (path: string | null) => void;
  toggleMainPageMode: () => void;
  setMainPageMode: (mode: MainPageMode) => void;
  resetMainPagemode: () => void;
  setOrientation: (orientation: Orientation) => void;
  setIsRoomSelectorOpen: (isOpen: boolean) => void;
}

export const useUIStore = create<UIState>()(
  devtools(
    (set, get) => ({
      redirectTo: null,
      mainPageMode: "roomLights",
      orientation: "landscape",
      isRoomSelectorOpen: false,
      setRedirectTo: (path) => {
        set({ redirectTo: path });
      },
      toggleMainPageMode: () => {
        get().mainPageMode === "roomLights"
          ? set({ mainPageMode: "allLights" })
          : set({ mainPageMode: "roomLights" });
      },
      setMainPageMode: (mode) =>
        set({ mainPageMode: mode }, false, "setMainPageMode"),
      resetMainPagemode: () =>
        set({ mainPageMode: "roomLights" }, false, "resetMainPagemode"),
      setOrientation: (orientation: Orientation) =>
        set({ orientation: orientation }, false, "setOrientation"),
      setIsRoomSelectorOpen: (isOpen: boolean) =>
        set({ isRoomSelectorOpen: isOpen }, false, "setIsRoomSelectorOpen"),
    }),
    { name: "UIStore" },
  ),
);
