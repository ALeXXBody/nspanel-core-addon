import { LoadingAnimation } from "@/components";
import { useConfigStore } from "@/stores";
import { useEffect } from "react";
import { useNavigate } from "react-router-dom";

function LoadingPage() {
  const isLoaded = useConfigStore((state) => state.isLoaded);
  const navigate = useNavigate();

  useEffect(() => {
    if (isLoaded) {
      navigate("mainpage");
    }
  }, [isLoaded]);

  if (!isLoaded)
    return (
      <div className={`relative z-10 h-full flex justify-center items-center`}>
        <div className="relative flex w-[80%] max-w-[455px] md:w-[40%] bg-bg rounded-xl border text-text/60 shadow-lg justify-center p-1">
          <LoadingAnimation />
          <span className="absolute bottom-5 text-text text-xs md:text-sm">
            Waiting for config from manager...
          </span>
        </div>
      </div>
    );
}

export default LoadingPage;
