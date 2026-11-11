import { useEffect, useRef, useState } from "react";

interface useLongPressLockConfig {
  delay?: number;
}

export function useLongPressLock({
  delay = 3000,
}: useLongPressLockConfig = {}) {
  const longPressTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [isLockActive, setIsLockActive] = useState(false);

  function startLock() {
    clearLock();
    setIsLockActive(true);
    longPressTimeout.current = setTimeout(() => {
      setIsLockActive(false);
      longPressTimeout.current = null;
    }, delay);
  }

  function clearLock() {
    if (longPressTimeout.current) {
      clearTimeout(longPressTimeout.current);
      longPressTimeout.current = null;
    }
    setIsLockActive(false);
  }

  function resetLockTimeout() {
    if (isLockActive && longPressTimeout.current) {
      clearTimeout(longPressTimeout.current);
      longPressTimeout.current = setTimeout(() => {
        setIsLockActive(false);
        longPressTimeout.current = null;
      }, delay);
    }
  }

  useEffect(() => {
    return () => {
      if (longPressTimeout.current) {
        clearTimeout(longPressTimeout.current);
        longPressTimeout.current = null;
      }
    };
  }, []);

  return {
    startLock,
    clearLock,
    resetLockTimeout,
    isLockActive,
  };
}
