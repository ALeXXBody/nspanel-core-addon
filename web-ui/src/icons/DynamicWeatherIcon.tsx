import { useEffect, useRef, useState } from "react";
import { LottieLight, type LottieHandle } from "lottie-react";

// Only the weather slugs actually used by ScreenSaverWeather, copied into the
// repo under src/assets/weather. The previous import.meta.glob over
// /node_modules/@meteocons/lottie/**/*.json matched ALL FOUR icon styles
// (~1900 files, ~34 MB of emitted chunks) while only 'fill' and ~25 slugs
// were ever rendered — and globbing an absolute node_modules path is fragile
// across package managers (breaks under pnpm's symlinked layout).
const iconModules = import.meta.glob<{ default: object }>(
  "@/assets/weather/*.json",
);

interface WeatherIconProps {
  slug: string;
  autoplay?: boolean;
  style?: "fill" | "line";
  size?: string | number;
}

function DynamicWeatherIcon({
  slug,
  autoplay,
  style = "fill",
  size = 64,
}: WeatherIconProps) {
  const [animationData, setAnimationData] = useState<object | null>(null);
  const lottieRef = useRef<LottieHandle>(null);

  useEffect(() => {
    const loader = iconModules[`/src/assets/weather/${slug}.json`];
    loader?.().then((mod) => setAnimationData(mod.default));
  }, [slug, style]);

  useEffect(() => {
    if (!autoplay) return;

    function pauseOnTouch() {
      lottieRef.current?.pause();
    }
    // Original behavior: pause on first user touch of the panel.
    document.addEventListener("touchstart", pauseOnTouch, {
      once: true,
      capture: true,
    });
    // Additional kiosk behavior: pause while the document is hidden instead
    // of keep animating (companion windows, backgrounded tabs).
    function pauseIfHidden() {
      if (document.hidden) {
        lottieRef.current?.pause();
      } else {
        lottieRef.current?.play();
      }
    }
    document.addEventListener("visibilitychange", pauseIfHidden);
    return () => {
      document.removeEventListener("touchstart", pauseOnTouch, {
        capture: true,
      });
      document.removeEventListener("visibilitychange", pauseIfHidden);
    };
  }, [autoplay]);

  if (!animationData) {
    return <div style={{ width: size }} />;
  }

  return (
    <LottieLight
      lottieRef={lottieRef}
      src={animationData}
      loop
      autoplay={autoplay}
      style={{ width: size, height: size }}
    />
  );
}

export default DynamicWeatherIcon;
