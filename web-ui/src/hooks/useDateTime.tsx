import { useConfigStore } from "@/stores";
import { useEffect, useState } from "react";

function useDateTime() {
  const [time, setTime] = useState(() => new Date());
  const isUsTime = useConfigStore(
    (state) => state.config?.clockUsStyle || false,
  );

  useEffect(() => {
    // Align to minute boundaries instead of ticking every second. Both
    // rendered strings (date + hh:mm) change at most once per minute, so
    // showing seconds was pure re-render churn on the whole page every
    // second on low-power wall-panel hardware.
    let timer: number;
    const schedule = () => {
      const msToMinute = 60001 - (Date.now() % 60000);
      timer = window.setTimeout(() => {
        setTime(new Date());
        schedule();
      }, msToMinute);
    };
    schedule();

    return () => clearTimeout(timer);
  }, []);

  const currentTime = time.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    hour12: isUsTime,
  });

  const currentDate = time.toLocaleDateString([], {
    weekday: "short",
    year: "numeric",
    month: "numeric",
    day: "numeric",
  });

  return { currentTime, currentDate };
}

export default useDateTime;
