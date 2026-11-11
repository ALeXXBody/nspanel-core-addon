import { useDateTime } from "@/hooks";
import { useUIStore } from "@/stores";
import { useWeatherStore } from "@/stores/useWeatherStore";
import { Icon } from "@mdi/react";
import {
  mdiHomeThermometerOutline,
  mdiThermometer,
  mdiUmbrellaOutline,
  mdiWeatherCloudyAlert,
  mdiWeatherSunsetDown,
  mdiWeatherSunsetUp,
  mdiWeatherWindy,
} from "@mdi/js";
import { DynamicWeatherIcon } from "@/icons";

// Static per-relation mapping (weather service code -> meteocons slug).
// Hoisted to module scope — this was rebuilt every render while the
// screensaver ticked once per second.
const weatherIconMap: Record<string, string> = {
  A: "clear-day",
  B: "clear-night",
  C: "partly-cloudy-day",
  D: "partly-cloudy-night",
  E: "haze",
  F: "cloudy",
  G: "fog",
  H: "hail",
  I: "rain",
  J: "drizzle",
  K: "partly-cloudy-day-rain",
  L: "snow",
  M: "overcast-snow",
  N: "sleet",
  O: "partly-cloudy-day-snow",
  P: "partly-cloudy-day-sleet",
  Q: "thunderstorms",
  R: "thunderstorms-rain",
  S: "thunderstorms-day",
  T: "wind",
  U: "extreme",
  V: "extreme-day",
  W: "umbrella",
  X: "sunset",
  Y: "sunrise",
};

function ScreenSaverWeather() {
  const { currentDate, currentTime } = useDateTime();
  // const showScreensaverOutsideTemperature = useConfigStore(
  //   (state) => state.config?.showScreensaverInsideTemperature,
  // );

  const weather = useWeatherStore((state) => state.weather);

  const orientation = useUIStore((state) => state.orientation);

  if (!weather) {
    return (
      <div className="flex flex-col w-full h-full justify-center items-center">
        <Icon path={mdiWeatherCloudyAlert} size={"100px"} />
        <div>Could not get weather...</div>
      </div>
    );
  }

  return (
    <div
      className={`grid w-full h-full ${orientation === "landscape" ? "grid-cols-2 grid-rows-2" : "grid-cols-1 grid-rows-[1fr_1fr_2fr]"} p-1 md:p2 gap-1 md:gap-2`}
    >
      {/* Top left contatiner */}
      <div className="flex flex-col w-full h-full rounded-xl bg-surface/20 border border-gray-500/20 shadow-md justify-between p-1">
        {/* First row TEMP and DATE */}
        <div className="flex justify-between">
          <div className="flex">
            <div className="flex items-center">
              <Icon path={mdiHomeThermometerOutline} size={"30px"} />
            </div>

            <div className="flex items-center">
              {weather.currentTemperatureString}
            </div>
          </div>

          <div className="flex items-center capitalize">
            <div className="truncate">{currentDate}</div>
          </div>
        </div>
        {/* Second row TIME */}
        <div className="flex flex-1 justify-center items-center text-7xl md:text-[120px]">
          <div className="">{currentTime}</div>
        </div>
        {/* Third row SUNRISE SUNSET */}
        <div className="flex w-full">
          <div className="flex flex-1 items-center justify-center">
            <Icon path={mdiWeatherSunsetUp} size={"30px"} />
            <span className="">{weather.sunriseString}</span>
          </div>
          <div className="flex flex-1 items-center justify-center">
            <Icon path={mdiWeatherSunsetDown} size={"30px"} />
            <span className="">{weather.sunsetString}</span>
          </div>
        </div>
      </div>
      {/* Top right container */}
      <div className="flex flex-col w-full h-full rounded-xl bg-surface/20 border border-gray-500/20 shadow-md p-1 items-center justify-between">
        {/* First row weather icon and current max min temp */}

        {orientation === "landscape" ? (
          <>
            <div className="flex h-full w-full justify-center overflow-hidden">
              <div className="flex flex-1 items-center justify-center">
                <DynamicWeatherIcon
                  slug={weatherIconMap[weather.currentWeatherIcon]}
                  autoplay
                  size={"100%"}
                />
              </div>
              <div className="flex flex-col flex-1 items-center justify-center">
                <div className="flex text-6xl md:text-8xl">
                  {weather.currentTemperatureString}
                </div>
                <div className="flex text-lg md:text-2xl">
                  {weather.currentMaxminTemperature}
                </div>
              </div>
            </div>
            {/* Second row precipitation and wind */}
            <div className="flex w-full">
              <div className="flex flex-1 items-center justify-center">
                <Icon path={mdiUmbrellaOutline} size={"30px"} />
                <div className="">{weather.currentPrecipitationString}</div>
              </div>
              <div className="flex flex-1 items-center justify-center">
                <Icon path={mdiWeatherWindy} size={"30px"} />
                <div className="">{weather.currentWindString}</div>
              </div>
            </div>
          </>
        ) : (
          <div className="flex h-full w-full items-center justify-center gap-6">
            {/* First column weather icon and temp */}
            <div className="flex items-center">
              <div className="flex">
                <DynamicWeatherIcon
                  slug={weatherIconMap[weather.currentWeatherIcon]}
                  size={"100%"}
                  autoplay={true}
                />
              </div>
              <div className="flex flex-col items-center">
                <div className="flex text-6xl md:text-8xl">
                  {weather.currentTemperatureString}
                </div>
                <div className="flex text-lg md:text-2xl">
                  {weather.currentMaxminTemperature}
                </div>
              </div>
            </div>
            {/* Second column precipitation and wind */}
            <div className="flex flex-col items-start">
              <div className="flex flex-1 items-center justify-center">
                <Icon path={mdiUmbrellaOutline} size={"30px"} />
                <div className="">{weather.currentPrecipitationString}</div>
              </div>
              <div className="flex flex-1 items-center justify-center">
                <Icon path={mdiWeatherWindy} size={"30px"} />
                <div className="">{weather.currentWindString}</div>
              </div>
            </div>
          </div>
        )}
      </div>
      {/* Bottom container */}
      <div
        className={`grid ${orientation === "landscape" ? "col-span-2" : ""} rounded-xl bg-surface/20 border border-gray-500/20 shadow-md`}
      >
        <div className="grid grid-cols-[min-content_repeat(5,minmax(0,1fr))] p-2">
          <div className="grid grid-rows-[1fr_50px_1fr_1fr_1fr] justify-center items-center">
            <div></div>
            <div></div>
            <Icon path={mdiThermometer} size={"20px"} />
            <Icon path={mdiUmbrellaOutline} size={"20px"} />
            <Icon path={mdiWeatherWindy} size={"20px"} />
          </div>
          {weather.forecastItems.map((day) => (
            <div
              key={day.displayString}
              className="grid grid-rows-[1fr_50px_1fr_1fr_1fr] items-center justify-center"
            >
              <div className="flex justify-center">{day.displayString}</div>
              <div className="flex justify-center">
                <DynamicWeatherIcon
                  slug={weatherIconMap[day.weatherIcon]}
                  size={"70px"}
                  autoplay={false}
                />
              </div>
              <div className="flex justify-center">
                {day.temperatureMaxminString}
              </div>
              <div className="flex justify-center">
                {day.precipitationString}
              </div>
              <div className="flex justify-center">{day.windString}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export default ScreenSaverWeather;
