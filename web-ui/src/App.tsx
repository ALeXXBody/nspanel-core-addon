// App.tsx
import { useEffect } from "react";
import { stompService } from "@/services/stompService";
import { useScreenOrientation } from "./hooks";
import { Outlet, useNavigate } from "react-router-dom";
import { useConfigStore, useIdleStore, useUIStore } from "./stores";
import { NSPanelConfig_NSPanelScreensaverMode } from "./generated/src/proto/protobuf_nspanel";

function App() {
  const navigate = useNavigate();
  const { redirectTo, setRedirectTo } = useUIStore();
  const screenSaverMode = useConfigStore(
    (state) => state.config?.screensaverMode,
  );

  const screensaverActivationTimeout = useConfigStore(
    (state) => state.config?.screensaverActivationTimeout,
  );
  const resetIdleTimer = useIdleStore.getState().resetIdleTimer;

  useEffect(() => {
    stompService.init();
    // resetIdleTimer(handleIdleTimeout);

    return () => {
      console.log("Cleaning up");
      stompService.cleanup(); // Total cleanup on unmount
    };
  }, []);

  // Screen wake lock: keeps wall-panel/tablet hosts from dimming/sleeping the
  // kiosk display, and reacquires the lock whenever visibility returns (browsers
  // drop the lock when the page is hidden). Best-effort — silently ignored on
  // hosts without the Screen Wake Lock API.
  useEffect(() => {
    let lock: WakeLockSentinel | null = null;
    let cancelled = false;

    const reacquire = async () => {
      if (cancelled) return;
      if (document.visibilityState === "visible" && "wakeLock" in navigator) {
        try {
          lock = (await navigator.wakeLock.request("screen")) as WakeLockSentinel;
          lock.addEventListener("release", () => {
            lock = null;
          });
        } catch {
          // Denied or unsupported; nothing we can do.
        }
      }
    };

    document.addEventListener("visibilitychange", reacquire);
    reacquire();

    return () => {
      cancelled = true;
      document.removeEventListener("visibilitychange", reacquire);
      lock?.release();
    };
  }, []);

  useEffect(() => {
    if (screensaverActivationTimeout) {
      resetIdleTimer(handleIdleTimeout);
    }
  }, [screensaverActivationTimeout]);

  useEffect(() => {
    if (redirectTo) {
      console.log("redirecting to", redirectTo);
      navigate(redirectTo);
      setRedirectTo(null);
    }
  }, [redirectTo]);

  useScreenOrientation();

  function handleIdleTimeout() {
    const currentPath = window.location.pathname;

    useConfigStore.getState().setDefaultRoom();
    useUIStore.getState().setMainPageMode("roomLights");
    useUIStore.setState({ isRoomSelectorOpen: false });
    if (
      screenSaverMode !== NSPanelConfig_NSPanelScreensaverMode.NO_SCREENSAVER &&
      useConfigStore.getState().config?.screensaverActivationTimeout !== 0
    ) {
      navigate("/webapp/screensaver");
    } else {
      if (currentPath === "/webapp" || currentPath === "/webapp/") {
      } else {
        navigate("/webapp/");
      }
    }
  }

  return (
    <div
      onPointerDownCapture={() => resetIdleTimer(handleIdleTimeout)}
      onPointerMoveCapture={() => resetIdleTimer(handleIdleTimeout)}
      className="relative h-[100dvh] w-full overflow-hidden bg-bg text-text"
    >
      {/*Background Layer*/}
      <div
        className="absolute inset-0 z-0 bg-cover bg-center opacity-100"
        style={{
          background: "linear-gradient(to top, #5c7852, #9fa0a8)",
        }}
      />
      <Outlet />
    </div>
  );
}
export default App;
