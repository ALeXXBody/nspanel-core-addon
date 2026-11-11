import { useUIStore } from "@/stores";
import { useEffect } from "react";

function useScreenOrientation() {
  const setOrientation = useUIStore.getState().setOrientation;

  useEffect(() => {
    function checkOrientation() {
      setOrientation(
        window.innerWidth >= window.innerHeight ? "landscape" : "portrait",
      );
    }
    checkOrientation();
    window.addEventListener("resize", checkOrientation);
    return () => window.removeEventListener("resize", checkOrientation);
  }, [setOrientation]);
}

export default useScreenOrientation;
