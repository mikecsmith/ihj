import { writable, derived, get } from "svelte/store";
import type { PopupMode, Toast, WorkItem } from "../types";
import { items } from "./items";

export const cursor = writable(0);
export const searchQuery = writable("");
export const popup = writable<PopupMode | null>(null);

let toastCounter = 0;
export const toasts = writable<Toast[]>([]);

export function showToast(message: string, type: Toast["type"] = "info", duration = 4000) {
  const id = ++toastCounter;
  toasts.update((t) => [...t, { message, type, id }]);
  setTimeout(() => {
    toasts.update((t) => t.filter((x) => x.id !== id));
  }, duration);
}

/** Fuzzy filter: checks if all chars of query appear in target in order. */
function fuzzyMatch(query: string, target: string): boolean {
  const ql = query.toLowerCase();
  const tl = target.toLowerCase();
  let qi = 0;
  for (let i = 0; i < tl.length && qi < ql.length; i++) {
    if (tl[i] === ql[qi]) qi++;
  }
  return qi === ql.length;
}

export const filtered = derived([items, searchQuery], ([$items, $query]) => {
  if (!$query) return $items;
  return $items.filter((item) =>
    fuzzyMatch($query, `${item.id} ${item.summary} ${item.fields?.assignee ?? ""} ${item.status} ${item.type}`),
  );
});

export const selectedItem = derived([filtered, cursor], ([$filtered, $cursor]) => {
  return $filtered[$cursor] ?? null;
});

export function moveCursor(delta: number) {
  const len = get(filtered).length;
  cursor.update((c) => Math.max(0, Math.min(len - 1, c + delta)));
}

export function setCursor(index: number) {
  cursor.set(index);
}
