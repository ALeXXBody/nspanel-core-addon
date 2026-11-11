import type { NSPanelWeatherUpdate } from "@/generated/src/proto/protobuf_nspanel";
import { create } from "zustand";
import { devtools } from "zustand/middleware";

interface WeatherState {
  weather: NSPanelWeatherUpdate | null;
  isLoaded: boolean;
  setWeather: (weatherData: NSPanelWeatherUpdate) => void;
  resetWeather: () => void;
}

export const useWeatherStore = create<WeatherState>()(
  devtools(
    (set) => ({
      weather: null,
      isLoaded: false,

      setWeather(weatherData) {
        set({ weather: weatherData, isLoaded: true }, false, "setWeather");
      },
      resetWeather() {
        set({ weather: null, isLoaded: false }, false, "resetWeather");
      },
    }),
    { name: "useWeatherStore" },
  ),
);
