import { create } from "zustand";
import { devtools, persist } from "zustand/middleware";

export type ThemeMode = "dark" | "light" | "system";

interface ThemeState {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
}

function applyTheme(mode: ThemeMode) {
  const resolved =
    mode === "system"
      ? window.matchMedia("(prefers-color-scheme: light)").matches
        ? "light"
        : "dark"
      : mode;
  document.documentElement.setAttribute("data-theme", resolved);
}

export const useThemeStore = create<ThemeState>()(
  devtools(
    persist(
      (set) => ({
        mode: "dark",
        setMode: (mode) => {
          applyTheme(mode);
          set({ mode }, false, "setMode");
        },
      }),
      {
        name: "nspanel-theme",
        onRehydrateStorage: () => (state) => {
          if (state) applyTheme(state.mode);
        },
      },
    ),
    { name: "ThemeStore" },
  ),
);

// Apply on first load (before React mounts) — called from main.tsx.
export function initTheme() {
  const stored = localStorage.getItem("nspanel-theme");
  let mode: ThemeMode = "dark";
  try {
    if (stored) mode = JSON.parse(stored)?.state?.mode ?? "dark";
  } catch {
    /* ignore */
  }
  applyTheme(mode);
}
