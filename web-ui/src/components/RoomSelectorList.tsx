import { useLongPress } from "@/hooks/useLongPress";
import { useConfigStore, useRoomsStore, useUIStore } from "@/stores";
import { LightType } from "@/types";
import { memo } from "react";
import { useNavigate } from "react-router-dom";
import { useShallow } from "zustand/shallow";
import CeilingLightIcon from "./CeilingLightIcon";
import TableLightIcon from "./TableLightIcon";

/**
 * One row of the room selector dialog.
 *
 * The row exists as its own component so `useLongPress` is called with a
 * STABLE hook count. Previously it was called inside a .map() after a
 * conditional return — if rooms streamed in incrementally while the dialog
 * was open, React would crash with "Rendered more hooks than during the
 * previous render". Each row now subscribes to only its own room entry, so
 * a room update also no longer re-renders every other row.
 */
const RoomSelectorRow = memo(function RoomSelectorRow({
  id,
  isActive,
}: {
  id: string;
  isActive: boolean;
}) {
  const navigate = useNavigate();
  const setIsOpen = useUIStore((state) => state.setIsRoomSelectorOpen);
  const setCurrentRoom = useConfigStore((state) => state.setCurrentRoom);
  const handleLightToggle = useRoomsStore.getState().handleLightToggle;

  const room = useRoomsStore((state) => state.rooms[id]);

  const selectRoomButtonHandlers = useLongPress({
    onShortPress: () => {
      setCurrentRoom(String(id));
      setIsOpen(false);
    },
    onLongPress: () => {
      setCurrentRoom(String(id));
      setIsOpen(false);
      navigate("/webapp/roompage");
    },
  });

  if (!room) return null; // Safe: no hooks are called after this point.

  return (
    <div
      key={id}
      {...selectRoomButtonHandlers}
      className={`flex h-[55px] md:h-[80px] justify-between items-center p-1 pl-2 md:p-2 md:pl-4 rounded-md transition-all duration-100 transform cursor-pointer ${
        isActive ? "bg-[#ffc101]" : "bg-surface/70 hover:bg-surface/50"
      }`}
    >
      {room.name}
      <div className="flex h-full items-center gap-2">
        <button
          onClick={(e) => {
            e.stopPropagation();
            handleLightToggle(LightType.CEILING, String(id));
          }}
          className={`flex h-full aspect-square rounded-md p-2 items-center justify-center cursor-pointer  border-1 border-white/30 ${
            isActive ? "bg-[#5c7852] hover:bg-[#5c7852]/90" : "hover:bg-text/10"
          }`}
        >
          <CeilingLightIcon
            isOn={room.numCeilingLightsOn > 0}
            className="h-full w-auto"
          />
        </button>
        <button
          onClick={(e) => {
            e.stopPropagation();
            handleLightToggle(LightType.TABLE, String(id));
          }}
          className={`flex h-full aspect-square rounded-md p-2 items-center justify-center cursor-pointer  border-1 border-white/30 ${
            isActive ? "bg-[#5c7852] hover:bg-[#5c7852]/90" : "hover:bg-text/10"
          }`}
        >
          <TableLightIcon
            isOn={room.numTableLightsOn > 0}
            className="h-full w-auto"
          />
        </button>
      </div>
    </div>
  );
});

function RoomSelectorList() {
  // Fetch JUST the string IDs. useShallow handles arrays of primitives perfectly!
  const roomIds = useRoomsStore(useShallow((state) => Object.keys(state.rooms)));
  const currentRoomId = useConfigStore((state) => state.currentRoomId);

  return (
    <>
      {roomIds.map((id) => (
        <RoomSelectorRow
          key={id}
          id={id}
          isActive={currentRoomId === String(id)}
        />
      ))}
    </>
  );
}

export default RoomSelectorList;
