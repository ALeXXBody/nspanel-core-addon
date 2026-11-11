import type { NSPanelRoomEntitiesPage } from "@/generated/src/proto/protobuf_nspanel";
import { create } from "zustand";
import { devtools } from "zustand/middleware";

interface ScenePagesState {
  scenePages: Record<string, NSPanelRoomEntitiesPage>;
  isLoaded: boolean;
  updateScenePage: (scenePageData: NSPanelRoomEntitiesPage) => void;
  resetScenePages: () => void;
}

export const useScenePagesStore = create<ScenePagesState>()(
  devtools(
    (set) => ({
      scenePages: {},
      isLoaded: false,

      updateScenePage: (scenePageData) => {
        set(
          (state) => ({
            scenePages: {
              ...state.scenePages,
              [String(scenePageData.id)]: scenePageData,
            },
            isLoaded: true,
          }),
          false,
          "updateScenePage",
        );
      },

      resetScenePages: () =>
        set(
          {
            scenePages: {},
            isLoaded: false,
          },
          false,
          "resetScenePages",
        ),
    }),
    { name: "ScenePageStore" },
  ),
);
