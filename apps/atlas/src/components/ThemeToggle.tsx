import type { ThemePreference } from "../lib/theme";

const OPTIONS: Array<{ value: ThemePreference; label: string; title: string }> = [
  { value: "light", label: "Light", title: "Always use the light theme" },
  { value: "dark", label: "Dark", title: "Always use the dark theme" },
  { value: "system", label: "Auto", title: "Follow your device's setting" },
];

/**
 * Three-way rather than a two-state switch: "auto" has to be reachable, or
 * choosing once permanently opts you out of following the OS.
 */
export default function ThemeToggle({
  preference,
  onChange,
}: {
  preference: ThemePreference;
  onChange: (next: ThemePreference) => void;
}) {
  return (
    <div className="theme-toggle" role="group" aria-label="Colour theme">
      {OPTIONS.map((o) => (
        <button
          key={o.value}
          type="button"
          title={o.title}
          aria-pressed={preference === o.value}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
