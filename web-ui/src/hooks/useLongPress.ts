import { useRef } from "react";

interface useLongPressConfig {
  onShortPress(): void;
  onLongPress(): void;
  delay?: number;
}

export function useLongPress({
  onShortPress,
  onLongPress,
  delay = 200,
}: useLongPressConfig) {
  const longPressTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // const longPressTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  function startPress(e: React.MouseEvent | React.TouchEvent) {
    if (e && e.type === "touchstart") {
      e.preventDefault();
    }
    longPressTimerRef.current = setTimeout(() => {
      onLongPress();
      longPressTimerRef.current = null;
    }, delay);
  }

  function endPress(e?: React.MouseEvent | React.TouchEvent) {
    if (e && e.type === "touchend") {
      e.preventDefault();
    }
    if (longPressTimerRef.current) {
      onShortPress();
      clearTimeout(longPressTimerRef.current);
      longPressTimerRef.current = null;
    }
  }

  function cancelPressOnScroll() {
    clearTimeout(longPressTimerRef.current || undefined);
    longPressTimerRef.current = null;
  }

  return {
    onMouseDown: startPress,
    onMouseUp: endPress,
    onMouseLeave: endPress,
    onTouchStart: startPress,
    onTouchEnd: endPress,
    onTouchMove: cancelPressOnScroll,
  };
}
