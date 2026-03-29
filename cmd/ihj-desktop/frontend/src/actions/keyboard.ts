/**
 * Keyboard handler — maps Alt+key and Ctrl+key combos to actions.
 * Mirrors the TUI key bindings from the Bubble Tea app.
 */

export interface ActionMap {
  refresh: () => void;
  filter: () => void;
  assign: () => void;
  transition: () => void;
  open: () => void;
  edit: () => void;
  comment: () => void;
  branch: () => void;
  extract: () => void;
  create: () => void;
  bulkEdit: () => void;
}

const ALT_BINDINGS: Record<string, keyof ActionMap> = {
  KeyR: "refresh",
  KeyF: "filter",
  KeyA: "assign",
  KeyT: "transition",
  KeyO: "open",
  KeyE: "edit",
  KeyC: "comment",
  KeyN: "branch",
  KeyX: "extract",
  KeyB: "bulkEdit",
};

export function setupKeyboard(actions: ActionMap) {
  function handler(e: KeyboardEvent) {
    // Alt+key actions
    if (e.altKey && !e.ctrlKey && !e.metaKey) {
      const action = ALT_BINDINGS[e.code];
      if (action && actions[action]) {
        e.preventDefault();
        e.stopPropagation();
        actions[action]();
        return;
      }
    }

    // Ctrl/Cmd+N = create
    if ((e.ctrlKey || e.metaKey) && e.code === "KeyN") {
      e.preventDefault();
      actions.create();
      return;
    }
  }

  document.addEventListener("keydown", handler);
  return () => document.removeEventListener("keydown", handler);
}
