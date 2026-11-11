import { ScreenSaverSimple, ScreenSaverWeather } from "@/components";
import { NSPanelConfig_NSPanelScreensaverMode } from "@/generated/src/proto/protobuf_nspanel";
import { useConfigStore } from "@/stores";
import { useNavigate } from "react-router-dom";

function ScreenSaverPage() {
  const navigate = useNavigate();

  const screensaverMode = useConfigStore(
    (state) => state.config?.screensaverMode,
  );

  const hasBackground =
    screensaverMode ===
      NSPanelConfig_NSPanelScreensaverMode.DATETIME_WITH_BACKGROUND ||
    screensaverMode ===
      NSPanelConfig_NSPanelScreensaverMode.WEATHER_WITH_BACKGROUND;

  const hasWeather =
    screensaverMode ===
      NSPanelConfig_NSPanelScreensaverMode.WEATHER_WITHOUT_BACKGROUND ||
    screensaverMode ===
      NSPanelConfig_NSPanelScreensaverMode.WEATHER_WITH_BACKGROUND;

  function handleWakeUp(e: React.MouseEvent) {
    e.stopPropagation();
    e.preventDefault();
    navigate("/webapp/");
  }

  return (
    <div
      onClick={handleWakeUp}
      onMouseMove={handleWakeUp}
      className={`relative z-10 h-full ${hasBackground ? "" : "bg-bg"} flex flex-col justify-center items-center`}
    >
      {hasWeather ? <ScreenSaverWeather /> : <ScreenSaverSimple />}
    </div>
  );
}

export default ScreenSaverPage;
