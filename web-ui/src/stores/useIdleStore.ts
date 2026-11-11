import { create } from "zustand";
import { devtools } from "zustand/middleware";
import { useConfigStore } from "./useConfigStore";

interface IdleState {
  timerId: ReturnType<typeof setTimeout> | null;
  lastResetTime: number;
  resetIdleTimer: (onTimeout: () => void) => void;
}

export const useIdleStore = create<IdleState>()(
  devtools(
    (set, get) => ({
      timerId: null,
      lastResetTime: 0,

      resetIdleTimer: (onTimeout) => {
        const { timerId, lastResetTime } = get();
        const now = Date.now();
        if (timerId && now - lastResetTime < 500) {
          return;
        }

        if (timerId) {
          clearTimeout(timerId);
        }

        const idleTimeoutMs =
          useConfigStore.getState().config?.screensaverActivationTimeout ??
          60000;
        const newTimer = setTimeout(() => {
          onTimeout();
        }, idleTimeoutMs);

        set({ timerId: newTimer, lastResetTime: now });
      },
    }),
    { name: "useIdleStore" },
  ),
);
