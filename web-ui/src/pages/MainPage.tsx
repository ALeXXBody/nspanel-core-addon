import { ChevronDown, SlidersVertical, Settings } from "lucide-react";
import { useConfigStore, useRoomsStore, useUIStore } from "@/stores";
import { useNavigate } from "react-router-dom";
import {
  CeilingLightIcon,
  ColorTempIcon,
  LoadingAnimation,
  RoomSelector,
  Slider,
  TableLightIcon,
} from "@/components";
import { LightType, SliderType } from "@/types";
import { useLongPress } from "@/hooks/useLongPress";
import { useLongPressLock } from "@/hooks/useLongPressLock";
import { IoSunny } from "react-icons/io5";
import { useDateTime } from "@/hooks";

function MainPage() {
  const mainPagemode = useUIStore((state) => state.mainPageMode);
  const { currentTime } = useDateTime();
  const navigate = useNavigate();

  const currentRoomId = useConfigStore((state) => state.currentRoomId);
  const isLoaded = useRoomsStore((state) => state.isLoaded);
  const room =
    mainPagemode === "roomLights"
      ? useRoomsStore((state) =>
          currentRoomId ? state.rooms[currentRoomId] : null,
        )
      : useRoomsStore((state) => state.globalRoom);

  const orientation = useUIStore((state) => state.orientation);
  const sliderOrientation =
    orientation === "landscape" ? "vertical" : "horizontal";
  const handleLightToggle = useRoomsStore.getState().handleLightToggle;
  const nextRoom = useConfigStore.getState().nextRoom;
  const toggleMainPageMode = useUIStore.getState().toggleMainPageMode;

  const tableLock = useLongPressLock();

  const tableButtonHandlers = useLongPress({
    onShortPress: () => {
      if (tableLock.isLockActive || ceilingLock.isLockActive) {
        clearAllLocks();
        return;
      }
      handleLightToggle(LightType.TABLE);
    },
    onLongPress: () => {
      if (tableLock.isLockActive) {
        clearAllLocks();
        return;
      }
      ceilingLock.clearLock();
      tableLock.startLock();
    },
  });

  const ceilingLock = useLongPressLock();

  const ceilingButtonHandlers = useLongPress({
    onShortPress: () => {
      if (tableLock.isLockActive || ceilingLock.isLockActive) {
        clearAllLocks();
        return;
      }
      handleLightToggle(LightType.CEILING);
    },
    onLongPress: () => {
      if (ceilingLock.isLockActive) {
        clearAllLocks();
        return;
      }
      tableLock.clearLock();
      ceilingLock.startLock();
    },
  });

  function clearAllLocks() {
    ceilingLock.clearLock();
    tableLock.clearLock();
  }

  const cardStyles = "rounded-xl bg-surface/20";

  const sliderStyles = `p-2 ${orientation === "landscape" ? "col-span-1" : "col-span-2"}`;

  const ceilingTableStyles = "w-[30px] md:w-[50px]";

  //Todo move this if statment down!
  if (!isLoaded || !room) {
    return (
      <div className={`relative z-10 h-full flex justify-center items-center`}>
        <div className="relative flex w-[80%] max-w-[455px] md:w-[40%] bg-bg rounded-xl border text-text/60 shadow-lg justify-center p-1">
          <LoadingAnimation />
          <span className="absolute bottom-5 text-text text-xs md:text-sm">
            Waiting for config from manager...
          </span>
        </div>
      </div>
    );
  }

  const activeLockBrightnessSlider = tableLock.isLockActive
    ? {
        type: LightType.TABLE,
        value: room.tableLightsDimLevel,
        reset: tableLock.resetLockTimeout,
      }
    : ceilingLock.isLockActive
      ? {
          type: LightType.CEILING,
          value: room.ceilingLightsDimLevel,
          reset: ceilingLock.resetLockTimeout,
        }
      : null;

  const activeLockColorTempSlider = tableLock.isLockActive
    ? {
        type: LightType.TABLE,
        value: room.tableLightsColorTemperatureValue,
        reset: tableLock.resetLockTimeout,
      }
    : ceilingLock.isLockActive
      ? {
          type: LightType.CEILING,
          value: room.ceilingLightsColorTemperatureValue,
          reset: ceilingLock.resetLockTimeout,
        }
      : null;

  return (
    <div
      key={currentRoomId}
      //Setting this onlick in the top div makes all button clicks except ceiling, table, brightness and colortemp clear active ceiling/table lock.
      //e.preventPropagation that is used on the ceiling, table, brightness, colortemp buttons prevent clearalllocks to be called.
      onClick={() => clearAllLocks()}
      className="relative z-10 grid h-full grid-rows-[auto_1fr_auto] gap-1 p-1 md:p-2 md:gap-2"
    >
      {/* ROW 1 */}
      <div
        className={`flex rounded-xl bg-surface/20 ${orientation === "landscape" ? "h-20 max-lg:[@media(min-aspect-ratio:2/1)]:h-10" : "h-20"}`}
      >
        <div className="flex flex-1 items-center justify-center h-full">
          <SlidersVertical />
        </div>
        <button
          onClick={(e) => {
            e.stopPropagation();
            navigate("settings");
          }}
          className="absolute top-2 right-2 z-20 p-1.5 rounded-full bg-surface/40 hover:bg-surface/70 text-text/70 hover:text-text transition-colors"
          aria-label="Settings"
        >
          <Settings className="w-4 h-4" />
        </button>
        <div className="flex flex-[7] items-center justify-end h-full p-5 text-lg md:text-xl">
          {currentTime}
        </div>
      </div>
      {/* ROW 2 Buttons: ceiling, table, brightness, colortemp */}
      <div
        className={`grid gap-1 md:gap-2 ${orientation === "landscape" ? "grid-cols-4 grid-rows-1" : "grid-cols-2 grid-rows-3"}`}
      >
        <div
          {...ceilingButtonHandlers}
          onClick={(e) => e.stopPropagation()}
          className={`grid grid-rows-[1fr_auto_1fr] cursor-pointer ${cardStyles}`}
        >
          <div className="flex items-end justify-center"></div>
          <div className={`flex flex-col items-center justify-center`}>
            <div
              className={`w-[60px] h-[3px] transition-colors duration-200 ${ceilingLock.isLockActive ? "bg-[#FFC101]" : "bg-transparent"} `}
            ></div>
            <CeilingLightIcon
              className={`${ceilingTableStyles}`}
              isOn={room.numCeilingLightsOn > 0}
            />
          </div>
          <div className="no-select flex items-start justify-center mt-4 md:mt-10 ">
            {room.ceilingLightsDimLevel}
          </div>
        </div>
        <div
          {...tableButtonHandlers}
          onClick={(e) => e.stopPropagation()}
          className={`grid grid-rows-[1fr_auto_1fr] cursor-pointer ${cardStyles}`}
        >
          <div className="flex items-end justify-center"></div>
          <div className={`flex flex-col items-center justify-center`}>
            <TableLightIcon
              className={`${ceilingTableStyles}`}
              isOn={room.numTableLightsOn > 0}
            />
            <div
              className={`w-[60px] h-[3px] transition-colors duration-200 ${tableLock.isLockActive ? "bg-[#FFC101]" : "bg-transparent"} `}
            ></div>
          </div>
          <div className="no-select flex items-start justify-center mt-4 md:mt-10">
            {room.tableLightsDimLevel}
          </div>
        </div>
        <div
          onClick={(e) => e.stopPropagation()}
          className={`flex flex-col ${cardStyles} ${sliderStyles}`}
        >
          <Slider
            value={
              activeLockBrightnessSlider
                ? activeLockBrightnessSlider.value
                : room.averageDimLevel
            }
            sliderType={SliderType.BRIGHTNESS}
            orientation={sliderOrientation}
            icon={<IoSunny size={"100%"} className="w-full" />}
            resetLockTimeout={activeLockBrightnessSlider?.reset}
            lockLightType={activeLockBrightnessSlider?.type}
          />
        </div>
        <div
          onClick={(e) => e.stopPropagation()}
          className={`flex flex-col ${cardStyles} ${sliderStyles}`}
        >
          <Slider
            value={
              activeLockColorTempSlider
                ? activeLockColorTempSlider.value
                : room.averageColorTemperature
            }
            sliderType={SliderType.COLORTEMP}
            orientation={sliderOrientation}
            icon={<ColorTempIcon />}
            resetLockTimeout={activeLockColorTempSlider?.reset}
            lockLightType={activeLockColorTempSlider?.type}
          />
        </div>
      </div>
      <div>
        {/* ROW 3 Room toggle button and Light Mode */}
        <div
          className={`grid grid-cols-2 gap-1 md:gap-2 ${orientation === "landscape" ? "h-20 max-lg:[@media(min-aspect-ratio:2/1)]:h-10" : "h-20"}`}
        >
          <div className="grid grid-cols-[1fr_2px_3fr] md:grid-cols-[1fr_4px_3fr]">
            <div
              className={`flex rounded-l-xl items-center justify-center bg-surface/20 active:opacity-60 duration-50 transition-all`}
            >
              <button
                onClick={
                  mainPagemode === "roomLights" ? () => nextRoom() : undefined
                }
                className="flex items-center justify-center h-full w-full cursor-pointer"
              >
                <ChevronDown />
              </button>
            </div>
            <div className="grid grid-rows-[1fr_60%_1fr]">
              <div className="bg-surface/20"></div>
              <div className="bg-transparent"></div>
              <div className="bg-surface/20"></div>
            </div>
            <div
              className={`flex rounded-r-xl items-center justify-center bg-surface/20 active:opacity-60 duration-50 transition-all`}
            >
              <RoomSelector />
            </div>
          </div>
          <button
            onClick={() => toggleMainPageMode()}
            className={`relative flex flex-col items-center justify-center w-full h-full cursor-pointer ${cardStyles} active:opacity-60 duration-50 transition-all`}
          >
            <div className="absolute left-0 top-0 bottom-0 w-[25%] flex items-center justify-center">
              <ChevronDown />
            </div>
            <div className="text-text/30">Mode</div>
            <span className="no-select text-md flex justify-center cursor-pointer">
              {mainPagemode === "roomLights" ? "Room Lights" : "All Lights"}
            </span>
          </button>
        </div>
      </div>
    </div>
  );
}

export default MainPage;
