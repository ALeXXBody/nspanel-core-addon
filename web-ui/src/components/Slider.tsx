import { useLightsStore, useRoomsStore } from "@/stores";
import { SliderType, type LightType } from "@/types";
import * as RadixSlider from "@radix-ui/react-slider";
import { useEffect, useState, type JSX } from "react";

interface SliderProps {
  value: number;
  sliderType: SliderType;
  orientation: "horizontal" | "vertical" | undefined;
  icon: JSX.Element;
  resetLockTimeout?(): void;
  lockLightType?: typeof LightType.CEILING | typeof LightType.TABLE;
  lightId?: string;
}

function Slider({
  value,
  sliderType,
  orientation,
  icon,
  resetLockTimeout,
  lockLightType,
  lightId,
}: SliderProps) {
  const [sliderValue, setSliderValue] = useState(value);

  useEffect(() => {
    setSliderValue(value);
  }, [value]);

  return (
    <RadixSlider.Root
      orientation={orientation}
      min={0}
      max={sliderType === SliderType.RGB ? 359 : 100}
      step={1}
      value={[sliderValue]}
      onValueChange={(values) => {
        setSliderValue(values[0]);
        lockLightType ? resetLockTimeout?.() : "";
      }}
      onValueCommit={(values) => {
        if (lightId) {
          useLightsStore
            .getState()
            .handleLightPageSlider(lightId, sliderType, values[0]);
        } else {
          useRoomsStore
            .getState()
            .handleMainPageLightSlider(values[0], sliderType, lockLightType);
        }
      }}
      className="relative flex flex-col items-center justify-start select-none touch-none w-full h-full"
    >
      {sliderType !== SliderType.RGB ? (
        <div className="absolute flex flex-col justify-center w-[30px] md:w-[50px] inset-0 m-auto pointer-events-none z-20">
          {icon}
        </div>
      ) : (
        <div
          className={`absolute flex flex-col items-center justify-center  pointer-events-none z-20 ${orientation === "vertical" ? "right-0 w-[10px] h-full rounded-r-xl" : "bottom-0 h-[10px] w-full rounded-b-xl"} `}
          style={{
            background: `linear-gradient(${orientation === "vertical" ? "to top" : "to right"} , hsl(0, 100%,70%), hsl(60, 100%,70%), hsl(120, 100%,70%), hsl(180, 100%,70%), hsl(240, 100%,70%), hsl(300, 100%,70%), hsl(360, 100%,70%))`,
          }}
        ></div>
      )}

      <RadixSlider.Track
        className={`relative h-full w-full rounded-xl overflow-hidden z-10`}
      >
        <RadixSlider.Range
          className={`absolute bg-surface/20 cursor-pointer transition-colors duration-200 ${orientation === "vertical" ? "w-full border-t-3" : "h-full border-r-3"} ${lockLightType ? "border-[#FFC101]" : ""} `}
        />
      </RadixSlider.Track>
      <RadixSlider.Thumb className="block" aria-label="Light brightness" />
    </RadixSlider.Root>
  );
}

export default Slider;
