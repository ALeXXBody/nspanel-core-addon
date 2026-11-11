import { useConfigStore, useRoomsStore, useUIStore } from "@/stores";
import * as Dialog from "@radix-ui/react-dialog";
import { useLongPress } from "@/hooks/useLongPress";
import { useNavigate } from "react-router-dom";
import RoomSelectorList from "./RoomSelectorList";

function RoomSelector() {
  const isOpen = useUIStore((state) => state.isRoomSelectorOpen);
  const setIsOpen = useUIStore((state) => state.setIsRoomSelectorOpen);

  const mainPagemode = useUIStore((state) => state.mainPageMode);
  const currentRoomId = useConfigStore((state) => state.currentRoomId);
  const currentRoomName = useRoomsStore((state) =>
    currentRoomId ? state.rooms[currentRoomId]?.name : undefined,
  );
  const orientation = useUIStore((state) => state.orientation);

  const navigate = useNavigate();
  const roomButtonHandlers = useLongPress({
    onShortPress: () => navigate("/webapp/roompage"),
    onLongPress: () => {
      setIsOpen(true);
    },
  });

  return (
    <Dialog.Root open={isOpen} onOpenChange={setIsOpen}>
      {/* <Dialog.Trigger asChild> */}
      <button
        {...roomButtonHandlers}
        disabled={mainPagemode !== "roomLights"}
        className="relative no-select text-md flex flex-col items-center justify-center w-full h-full cursor-pointer"
      >
        <div className="text-text/30">Room</div>
        <div>{mainPagemode === "roomLights" ? currentRoomName : "All"}</div>
      </button>
      {/* </Dialog.Trigger> */}
      <Dialog.Portal>
        <Dialog.Overlay className="z-50 fixed inset-0 backdrop-blur data-[state=open]:animate-overlayShow" />
        <div className="fixed inset-0 z-50 flex items-end justify-center p-2">
          <Dialog.Title className="m-0"></Dialog.Title>
          <Dialog.Description />
          <Dialog.Content
            className={`z-50 no-select ${orientation === "landscape" ? "grid grid-cols-2" : "flex flex-col"} gap-[4px] grow rounded-xl text-text p-1 md:p-4 shadow-[var(--shadow-6)] focus:outline-none data-[state=open]:animate-contentShow`}
          >
            <RoomSelectorList />
          </Dialog.Content>
        </div>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

export default RoomSelector;
