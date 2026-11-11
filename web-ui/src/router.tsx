import { createBrowserRouter } from "react-router-dom";
import { lazy, Suspense } from "react";
import App from "./App";
import { LoadingAnimation } from "@/components";

// Route-level code splitting. Every page was eager via the barrel export,
// so the loading screen loaded the entire app before it could even show
// the animation. MainPage + sliders is by far the biggest page.
const MainPage = lazy(() => import("@/pages/MainPage"));
const RoomPage = lazy(() => import("@/pages/RoomPage"));
const LightPage = lazy(() => import("@/pages/LightPage"));
const ScreenSaverPage = lazy(() => import("@/pages/ScreenSaverPage"));
const SettingsPage = lazy(() => import("@/pages/SettingsPage"));
const LoadingPage = lazy(() => import("@/pages/LoadingPage"));

function LazyPage({ children }: { children: React.ReactNode }) {
  return (
    <Suspense
      fallback={
        <div className="flex h-dvh w-full items-center justify-center">
          <LoadingAnimation />
        </div>
      }
    >
      {children}
    </Suspense>
  );
}

export const router = createBrowserRouter([
  {
    path: "/webapp",
    element: <App />,
    children: [
      { index: true, element: <LazyPage><LoadingPage /></LazyPage> },
      { path: "mainpage", element: <LazyPage><MainPage /></LazyPage> },
      { path: "roompage", element: <LazyPage><RoomPage /></LazyPage> },
      { path: "lightpage/:id", element: <LazyPage><LightPage /></LazyPage> },
      { path: "screensaver", element: <LazyPage><ScreenSaverPage /></LazyPage> },
      { path: "settings", element: <LazyPage><SettingsPage /></LazyPage> },
    ],
  },
]);
