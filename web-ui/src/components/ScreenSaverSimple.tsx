import { useDateTime } from "@/hooks";
import { useConfigStore } from "@/stores";
import { useWeatherStore } from "@/stores/useWeatherStore";
import { FaTemperatureHalf } from "react-icons/fa6";

function ScreenSaverSimple() {
  const { currentDate, currentTime } = useDateTime();
  const showScreensaverOutsideTemperature = useConfigStore(
    (state) => state.config?.showScreensaverInsideTemperature,
  );

  const weather = useWeatherStore((state) => state.weather);

  return (
    <>
      <div className="text-8xl">{currentTime}</div>
      <div className="capitalize">{currentDate}</div>
      <div className="flex items-center gap-2 absolute top-10 left-10 text-xl">
        <FaTemperatureHalf />
        {showScreensaverOutsideTemperature && weather?.currentTemperatureString}
      </div>
    </>
  );
}

export default ScreenSaverSimple;
