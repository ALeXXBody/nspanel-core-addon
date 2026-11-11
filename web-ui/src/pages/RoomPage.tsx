import {
  NSPanelRoomEntitiesPage_EntitySlot,
  NSPanelRoomEntitiesPage_EntitySlot_EntityType,
} from "@/generated/src/proto/protobuf_nspanel";
import { stompService } from "@/services/stompService";
import {
  useConfigStore,
  useEntityPagesStore,
  useRoomsStore,
  useUIStore,
} from "@/stores";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import {
  BsFillQuestionOctagonFill,
  BsLightbulbFill,
  BsToggleOn,
} from "react-icons/bs";

import { useNavigate } from "react-router-dom";

function RoomPage() {
  const navigate = useNavigate();
  const orientation = useUIStore((state) => state.orientation);
  const nextRoom = useConfigStore((state) => state.nextRoom);
  const prevRoom = useConfigStore((state) => state.prevRoom);

  const currentRoomId = useConfigStore((state) => state.currentRoomId);

  const entityPageIds = useRoomsStore((state) =>
    currentRoomId ? state.rooms[currentRoomId]?.entityPageIds : undefined,
  );
  const [entityIndex, setEntityIndex] = useState(0);

  const activeEntityPageId = entityPageIds?.[entityIndex];
  const activeEntityPage = useEntityPagesStore((state) =>
    activeEntityPageId ? state.entityPages[activeEntityPageId] : undefined,
  );
  const totalEntities = activeEntityPage?.pageType || 8;
  const headerText = activeEntityPage?.headerText || undefined;
  const entities = activeEntityPage?.entities || [];

  const roomViewMap: Map<number, NSPanelRoomEntitiesPage_EntitySlot> =
    new Map();
  entities.forEach((entity) => {
    roomViewMap.set(entity.roomViewPosition, entity);
  });

  const roomEntities = Array.from(
    { length: totalEntities },
    (_, index) => index,
  );

  const iconMap: Record<
    NSPanelRoomEntitiesPage_EntitySlot_EntityType,
    ReactNode
  > = {
    [NSPanelRoomEntitiesPage_EntitySlot_EntityType.ENTITY_TYPE_BUTTON]: (
      <BsFillQuestionOctagonFill />
    ),
    [NSPanelRoomEntitiesPage_EntitySlot_EntityType.ENTITY_TYPE_LIGHT]: (
      <BsLightbulbFill size={22} />
    ),
    [NSPanelRoomEntitiesPage_EntitySlot_EntityType.ENTITY_TYPE_SCENE]: (
      <BsFillQuestionOctagonFill />
    ),
    [NSPanelRoomEntitiesPage_EntitySlot_EntityType.ENTITY_TYPE_SWITCH]: (
      <BsToggleOn size={22} />
    ),
    [NSPanelRoomEntitiesPage_EntitySlot_EntityType.ENTITY_TYPE_THERMOSTAT]: (
      <BsFillQuestionOctagonFill />
    ),
    [NSPanelRoomEntitiesPage_EntitySlot_EntityType.ENTITY_TYPE_UNSPECIFIED]: (
      <BsFillQuestionOctagonFill />
    ),
    [NSPanelRoomEntitiesPage_EntitySlot_EntityType.UNRECOGNIZED]: (
      <BsFillQuestionOctagonFill />
    ),
  };

  useEffect(() => {
    if (
      useConfigStore.getState().roomChangeDirection === "DIRECT" ||
      useConfigStore.getState().roomChangeDirection === "NEXT"
    ) {
      setEntityIndex(0);
    } else {
      if (entityPageIds) {
        setEntityIndex(entityPageIds?.length - 1);
      }
    }
  }, [currentRoomId]);

  function handleNextPage() {
    if (entityPageIds) {
      if (entityIndex + 1 < entityPageIds.length) {
        setEntityIndex(entityIndex + 1);
      } else {
        console.log("nextroom");
        nextRoom();
      }
    }
  }

  function handlePrevPage() {
    if (entityPageIds) {
      if (entityIndex !== 0) {
        setEntityIndex(entityIndex - 1);
      } else {
        prevRoom();
      }
    }
  }

  return (
    <div className="relative z-10 h-full grid grid-rows-[auto_1fr] gap-1 p-1 md:p-2 md:gap-2">
      {/* ROW 1 NAVIGATION AND HEADER TEXT*/}
      <div
        className={`grid grid-cols-[auto_2px_auto_1fr_auto] md:grid-cols-[auto_4px_auto_1fr_auto] rounded-xl ${orientation === "landscape" ? "h-20 max-lg:[@media(min-aspect-ratio:2/1)]:h-10" : "h-20"}`}
      >
        <div
          onClick={() =>
            // (useConfigStore.setState({ resetDefaultRoom: true }),
            navigate("/webapp/")
          }
          className="flex p-5 h-full rounded-l-xl items-center bg-surface/20 justify-center select-none cursor-pointer active:opacity-60 duration-50 transition-all"
        >
          <ChevronLeft />
        </div>
        <div className="grid grid-rows-[1fr_60%_1fr]">
          <div className="bg-surface/20"></div>
          <div className="bg-transparent"></div>
          <div className="bg-surface/20"></div>
        </div>
        <div
          onClick={() => handlePrevPage()}
          className="flex p-5 h-full items-center bg-surface/20 justify-center select-none cursor-pointer active:opacity-60 duration-50 transition-all"
        >
          <ChevronLeft />
        </div>
        <div className="flex h-full items-center justify-center bg-surface/20 select-none">
          {headerText}
        </div>
        <div
          onClick={() => handleNextPage()}
          className="flex p-5 h-full rounded-r-xl items-center bg-surface/20 justify-center select-none cursor-pointer active:opacity-60 duration-50 transition-all"
        >
          <ChevronRight />
        </div>
      </div>
      {/* Entity buttons */}
      <div
        className={`grid grid-cols-2 gap-1 md:gap-2 ${totalEntities === 4 ? "grid-rows-2" : totalEntities === 8 ? "grid-rows-4" : "grid-rows-6"}`}
      >
        {roomEntities.map((roomEntity) => {
          const entity = roomViewMap.get(roomEntity);
          const isOn = entity?.icon === "s";
          const entityType = entity?.type;

          const icon = entityType ? iconMap[entityType] : null;

          return (
            <div
              key={entity?.id}
              className={`grid ${totalEntities !== 4 ? "grid-cols-[1fr_2px_70px] md:grid-cols-[1fr_2px_100px]" : "grid-rows-[1fr_2px_1fr]"} items-center rounded-xl`}
            >
              <button
                onClick={() =>
                  // (useConfigStore.setState({ resetDefaultRoom: true }),
                  {
                    entity?.type ===
                    NSPanelRoomEntitiesPage_EntitySlot_EntityType.ENTITY_TYPE_LIGHT
                      ? navigate(`/webapp/lightpage/${entity?.id}`)
                      : "";
                  }
                }
                className={`relative flex h-full bg-surface/20 items-center ${totalEntities !== 4 ? "justify-start rounded-l-xl p-3" : "justify-center rounded-t-xl"} md:text-xl select-none cursor-pointer active:opacity-60 duration-50 transition-all`}
              >
                {entity?.name}
                {entity && icon ? (
                  <div className="absolute top-2 left-2 md:top-3 md:left-3">
                    {icon}
                  </div>
                ) : (
                  ""
                )}
              </button>
              <div
                className={`grid ${totalEntities !== 4 ? "grid-rows-[1fr_60%_1fr]" : "grid-cols-[1fr_60%_1fr]"} h-full`}
              >
                <div className="bg-surface/20"></div>
                <div className="bg-transparent"></div>
                <div className="bg-surface/20"></div>
              </div>
              <div
                onClick={() => {
                  entity && activeEntityPageId
                    ? stompService.sendToggleEntityFromEntitiesPage(
                        activeEntityPageId,
                        roomEntity,
                      )
                    : "";
                }}
                className={`flex h-full bg-surface/20 justify-center items-center ${totalEntities !== 4 ? "rounded-r-xl" : "rounded-b-xl"} ${entity ? "cursor-pointer" : ""}`}
              >
                {entity ? (
                  <div
                    className={`relative inline-flex h-6 w-10 md:h-7 md:w-12 rounded-full ${isOn ? "bg-[#ffc101] justify-end" : "bg-text/40 justify-start"}  p-1 transition-colors duration-200 ease-in-out select-none focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400`}
                  >
                    <span
                      className={`h-4 w-4 md:h-5 md:w-5 inline-block rounded-full bg-white shadow-md transform transition-transform duration-200 ease-in-out`}
                    ></span>
                  </div>
                ) : (
                  ""
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default RoomPage;
