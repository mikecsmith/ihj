import { writable } from "svelte/store";

type Theme = "dark" | "light";

const stored = (typeof localStorage !== "undefined" && localStorage.getItem("ihj-theme")) as Theme | null;
export const theme = writable<Theme>(stored || "dark");

theme.subscribe((value) => {
  if (typeof document !== "undefined") {
    document.documentElement.dataset.theme = value;
  }
  if (typeof localStorage !== "undefined") {
    localStorage.setItem("ihj-theme", value);
  }
});

export function toggleTheme() {
  theme.update((t) => (t === "dark" ? "light" : "dark"));
}
