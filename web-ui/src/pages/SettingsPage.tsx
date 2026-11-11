import { useThemeStore, type ThemeMode } from "@/stores/useThemeStore";
import { ChevronLeft } from "lucide-react";
import { useNavigate } from "react-router-dom";

const themes: { id: ThemeMode; label: string; preview: { bg: string; surface: string; text: string; accent: string } }[] = [
  { id: "dark", label: "Dark", preview: { bg: "#000000", surface: "#1a1a1a", text: "#ffffff", accent: "#3b82f6" } },
  { id: "light", label: "Light", preview: { bg: "#f5f5f5", surface: "#ffffff", text: "#111111", accent: "#2563eb" } },
  { id: "system", label: "System", preview: { bg: "#808080", surface: "#cccccc", text: "#111111", accent: "#3b82f6" } },
];

function ThemePreview({ preview }: { preview: (typeof themes)[0]["preview"] }) {
  return (
    <div
      className="w-full aspect-[4/3] rounded-lg overflow-hidden border border-white/20 shadow-md"
      style={{ backgroundColor: preview.bg }}
    >
      <div className="h-1/3 flex items-center px-2 gap-1" style={{ backgroundColor: preview.surface }}>
        <div className="w-2 h-2 rounded-full" style={{ backgroundColor: preview.accent }} />
        <div className="h-1.5 w-8 rounded" style={{ backgroundColor: preview.text, opacity: 0.8 }} />
      </div>
      <div className="p-2 flex flex-col gap-1">
        <div className="h-1.5 w-3/4 rounded" style={{ backgroundColor: preview.text, opacity: 0.6 }} />
        <div className="h-1.5 w-1/2 rounded" style={{ backgroundColor: preview.text, opacity: 0.4 }} />
        <div className="mt-1 h-3 w-8 rounded" style={{ backgroundColor: preview.accent }} />
      </div>
    </div>
  );
}

export default function SettingsPage() {
  const mode = useThemeStore((s) => s.mode);
  const setMode = useThemeStore((s) => s.setMode);
  const navigate = useNavigate();

  return (
    <div className="h-dvh w-full bg-bg text-text flex flex-col">
      <div className="flex items-center gap-2 p-3 border-b border-border/20">
        <button onClick={() => navigate(-1)} className="p-1 rounded hover:bg-surface/50">
          <ChevronLeft className="w-5 h-5" />
        </button>
        <h1 className="text-lg font-semibold">Settings</h1>
      </div>
      <div className="flex-1 overflow-auto p-4">
        <h2 className="text-sm font-semibold text-text/60 mb-2 uppercase tracking-wide">Theme</h2>
        <div className="grid grid-cols-3 gap-3">
          {themes.map((t) => {
            const active = mode === t.id;
            return (
              <button
                key={t.id}
                onClick={() => setMode(t.id)}
                className={`flex flex-col gap-2 p-2 rounded-xl border transition-all ${
                  active ? "border-accent ring-2 ring-accent/40" : "border-border/20 hover:border-border/40"
                }`}
              >
                <ThemePreview preview={t.preview} />
                <span className="text-xs font-medium text-center">{t.label}</span>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
