import { LottieLight, LottieSubscription, type LottieHandle } from "lottie-react";
import animationData from "@/animations/nspanelAnimation.json";
import { useRef } from "react";

// Uses the light SVG engine build (~94 kB instead of the full 300 kB
// lottie-web). Previously this alone pulled in
// @lottiefiles/react-lottie-player, which ships its own full lottie-web copy —
// 801 KB of the 1.6 MB main bundle for one loading spinner.
export default function LoadingAnimation() {
  const lottieInstanceRef = useRef<LottieHandle>(null);

  const subscriptions = useRef<Partial<Record<"ready" | "complete", () => void>>>({
    [LottieSubscription.ready]: () => {
      lottieInstanceRef.current?.playSegments([0, 47]);
    },
    [LottieSubscription.complete]: () => {
      lottieInstanceRef.current?.playSegments([48, 59]);
    },
  }).current;

  return (
    <LottieLight
      lottieRef={(instance) => {
        lottieInstanceRef.current = instance;
      }}
      src={animationData}
      subscriptions={subscriptions}
      autoplay={false}
      loop={false}
      className="w-full h-auto]"
    />
  );
}
