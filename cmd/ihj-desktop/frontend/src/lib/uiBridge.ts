/**
 * UI Bridge — connects Go commands.UI events to the Svelte popup system.
 *
 * The Go DesktopUI emits Wails events when a command needs user input.
 * This module subscribes to those events, opens the appropriate popup,
 * and calls the corresponding Resolve* binding when the user responds.
 */

import { get } from "svelte/store";
import { EventsOn } from "./wailsjs/runtime/runtime";
import { popup } from "./stores/ui";
import { showToast } from "./stores/ui";
import { workspace, fieldDefs } from "./stores/items";
import {
  resolveSelect,
  resolveConfirm,
  resolveEditText,
  resolveEditDocument,
  resolvePromptText,
  resolveReviewDiff,
} from "./bindings";
import type { FieldDiff } from "./types";

/** Call once from App.svelte onMount to start listening for UI events. */
export function setupUIBridge(): () => void {
  const cleanups: Array<() => void> = [];

  // ── Fire-and-forget: notifications & status ──

  cleanups.push(
    EventsOn("ui:notify", (data: { title: string; message: string }) => {
      const type = data.title.toLowerCase().includes("error") || data.title.toLowerCase().includes("fail")
        ? "error"
        : data.title.toLowerCase().includes("warning")
          ? "info"
          : "success";
      showToast(`${data.title}: ${data.message}`, type);
    }),
  );

  cleanups.push(
    EventsOn("ui:status", (_data: { message: string }) => {
      // Status messages are transient — could wire to a status bar in future.
      // For now they're silent; Notify handles user-facing messages.
    }),
  );

  // ── Interactive: Select ──

  cleanups.push(
    EventsOn("ui:select", (data: { title: string; options: string[] }) => {
      popup.set({
        kind: "select",
        title: data.title,
        options: data.options,
        onResult: (index: number | null) => {
          resolveSelect(index ?? -1);
        },
      });
    }),
  );

  // ── Interactive: Confirm ──

  cleanups.push(
    EventsOn("ui:confirm", (data: { prompt: string }) => {
      popup.set({
        kind: "select",
        title: data.prompt,
        options: ["Yes", "No"],
        onResult: (index: number | null) => {
          resolveConfirm(index === 0);
        },
      });
    }),
  );

  // ── Interactive: EditText (raw/fallback BulkEditor) ──

  cleanups.push(
    EventsOn("ui:edittext", (data: { initial: string }) => {
      popup.set({
        kind: "editor",
        title: "Edit",
        content: data.initial,
        onResult: (result) => {
          if (result && result.action === "save") {
            resolveEditText(result.content, false);
          } else {
            resolveEditText("", true);
          }
        },
      });
    }),
  );

  // ── Interactive: EditDocument (structured form) ──

  cleanups.push(
    EventsOn("ui:editdocument", (data: { metadata: Record<string, string>; description: string }) => {
      const ws = get(workspace);
      const defs = get(fieldDefs);
      const title = data.metadata["summary"]
        ? `Edit: ${data.metadata["summary"]}`
        : "Edit";
      popup.set({
        kind: "form",
        title,
        metadata: data.metadata,
        description: data.description,
        fields: defs,
        statuses: ws?.statuses ?? [],
        types: ws?.types.map((t) => t.name) ?? [],
        onResult: (result) => {
          if (result) {
            resolveEditDocument(result.metadata, result.description, false);
          } else {
            resolveEditDocument({}, "", true);
          }
        },
      });
    }),
  );

  // ── Interactive: PromptText ──

  cleanups.push(
    EventsOn("ui:prompt", (data: { prompt: string }) => {
      popup.set({
        kind: "input",
        title: data.prompt,
        placeholder: "Enter text...",
        onResult: (text: string | null) => {
          resolvePromptText(text ?? "", text === null);
        },
      });
    }),
  );

  // ── Interactive: ReviewDiff ──

  cleanups.push(
    EventsOn(
      "ui:reviewdiff",
      (data: { title: string; changes: FieldDiff[]; options: string[] }) => {
        popup.set({
          kind: "reviewdiff",
          title: data.title,
          changes: data.changes,
          options: data.options,
          onResult: (index: number | null) => {
            resolveReviewDiff(index ?? -1);
          },
        });
      },
    ),
  );

  return () => {
    for (const fn of cleanups) fn();
  };
}
