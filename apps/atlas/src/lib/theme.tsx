import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

export type Theme = "light" | "dark";
export type ThemePreference = Theme | "system";

const STORAGE_KEY = "flockwatch-theme";

/**
 * Reads the stored preference.
 *
 * Wrapped in try/catch because localStorage throws outright (rather than
 * returning null) in some privacy configurations — Safari private mode and
 * hardened profiles among them. This site's readers are likelier than most
 * to run those, and a theme preference is never worth breaking the page for.
 */
function readPreference(): ThemePreference {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark" || stored === "system") return stored;
  } catch {
    // Storage unavailable; fall through to the OS setting.
  }
  return "system";
}

function systemIsDark(): boolean {
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false;
}

interface ThemeValue {
  preference: ThemePreference;
  setPreference: (p: ThemePreference) => void;
  /** The theme actually on screen, with "system" already resolved. */
  active: Theme;
}

const ThemeContext = createContext<ThemeValue | null>(null);

/**
 * Single source of truth for the theme.
 *
 * Deliberately a context rather than a plain hook: the header toggle and the
 * map both need it, and two independent useState instances don't sync —
 * toggling the header left the map still requesting dark basemap tiles on a
 * light page.
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreference] = useState<ThemePreference>(readPreference);
  const [osDark, setOsDark] = useState(systemIsDark);

  useEffect(() => {
    const root = document.documentElement;
    // "system" removes the attribute rather than writing a concrete value,
    // so the CSS media query stays in charge if the OS changes later.
    if (preference === "system") root.removeAttribute("data-theme");
    else root.setAttribute("data-theme", preference);

    try {
      localStorage.setItem(STORAGE_KEY, preference);
    } catch {
      // Preference just won't persist across reloads.
    }
  }, [preference]);

  useEffect(() => {
    const mq = window.matchMedia?.("(prefers-color-scheme: dark)");
    if (!mq) return;
    const onChange = (e: MediaQueryListEvent) => setOsDark(e.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const active: Theme = preference === "system" ? (osDark ? "dark" : "light") : preference;

  return (
    <ThemeContext.Provider value={{ preference, setPreference, active }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme(): ThemeValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used inside <ThemeProvider>");
  return ctx;
}
