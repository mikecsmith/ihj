import { writable, derived } from "svelte/store";
import type { WorkItem, Workspace, FieldDef } from "../types";

/** Raw tree roots from the backend (sorted, hierarchical). */
export const roots = writable<WorkItem[]>([]);
export const workspace = writable<Workspace | null>(null);
export const fieldDefs = writable<FieldDef[]>([]);
export const filter = writable("active");
export const fetchedAt = writable<Date | null>(null);

/** Flatten a tree of WorkItems into a display list with depth annotations. */
function flattenTree(nodes: WorkItem[], depth: number): WorkItem[] {
  const result: WorkItem[] = [];
  for (const node of nodes) {
    result.push({ ...node, depth });
    if (node.children?.length > 0) {
      result.push(...flattenTree(node.children, depth + 1));
    }
  }
  return result;
}

/** Flat, sorted list of all items with depth for tree display. */
export const items = derived(roots, ($roots) => flattenTree($roots, 0));

/** Build a lookup registry from the flat items list. */
export const registry = derived(items, ($items) => {
  const map = new Map<string, WorkItem>();
  for (const item of $items) {
    map.set(item.id, item);
  }
  return map;
});
