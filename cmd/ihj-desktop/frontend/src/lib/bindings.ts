/**
 * Bindings layer — delegates to Wails-generated bindings when available,
 * falls back to stubs for standalone frontend development.
 */

import type { WorkItem, Workspace, FieldDef } from "./types";

// Detect Wails runtime — available when running inside the desktop app.
const isWails = typeof window !== "undefined" && (window as any).go?.desktop?.App;

// ── Wails imports (lazy, only resolved when running in Wails) ──

let wailsApp: any = null;

function app(): any {
  if (!wailsApp && isWails) {
    wailsApp = (window as any).go.desktop.App;
  }
  return wailsApp;
}

// ── Workspace & data ──

export async function getWorkspace(): Promise<Workspace> {
  if (app()) return app().GetWorkspace();
  return {
    slug: "demo-board",
    name: "Demo Board",
    provider: "demo",
    baseUrl: "",
    types: [
      { id: 1, name: "Feature", order: 20, color: "magenta", hasChildren: true, template: "" },
      { id: 3, name: "Task", order: 30, color: "default", hasChildren: true, template: "" },
      { id: 5, name: "Subtask", order: 40, color: "white", hasChildren: false, template: "" },
      { id: 7, name: "Bug", order: 25, color: "red", hasChildren: false, template: "" },
    ],
    statuses: ["Backlog", "To Do", "In Progress", "In Review", "Done"],
    filters: { active: "statusCategory != Done", mine: "assignee = currentUser()", all: "" },
  };
}

export async function searchItems(filter: string, noCache = false): Promise<WorkItem[]> {
  if (app()) return app().Search(filter, noCache);
  return [];
}

export async function getFieldDefs(): Promise<FieldDef[]> {
  if (app()) return app().FieldDefinitions();
  return [
    { key: "priority", label: "Priority", type: "enum", enum: ["Highest", "High", "Medium", "Low", "Lowest"], visibility: "default", topLevel: true },
    { key: "assignee", label: "Assignee", type: "string", enum: [], visibility: "default", topLevel: true },
    { key: "labels", label: "Labels", type: "string_array", enum: [], visibility: "default", topLevel: true },
    { key: "components", label: "Components", type: "string_array", enum: [], visibility: "default", topLevel: true },
  ];
}

// ── Export ──

export async function exportManifest(filter: string, full: boolean): Promise<string> {
  if (app()) return app().ExportManifest(filter, full);
  return "";
}

export async function getManifestSchema(): Promise<object> {
  if (app()) return app().ManifestSchema();
  return {};
}

// ── Command-delegating methods ──
// These call commands.* on the Go side, which drives UI interaction via
// the DesktopUI event bridge.

export async function runAssign(id: string): Promise<void> {
  if (app()) return app().RunAssign(id);
}

export async function runTransition(id: string): Promise<void> {
  if (app()) return app().RunTransition(id);
}

export async function runCreate(): Promise<void> {
  if (app()) return app().RunCreate();
}

export async function runEdit(id: string): Promise<void> {
  if (app()) return app().RunEdit(id);
}

export async function runComment(id: string): Promise<void> {
  if (app()) return app().RunComment(id);
}

export async function runBranch(id: string): Promise<void> {
  if (app()) return app().RunBranch(id);
}

export async function runExtract(id: string): Promise<void> {
  if (app()) return app().RunExtract(id);
}

export async function runApplyManifest(yaml: string): Promise<void> {
  if (app()) return app().RunApplyManifest(yaml);
}

// ── UI Bridge Resolve Methods ──
// Called by the uiBridge to unblock pending Go UI calls.

export async function resolveSelect(index: number): Promise<void> {
  if (app()) return app().ResolveSelect(index);
}

export async function resolveConfirm(yes: boolean): Promise<void> {
  if (app()) return app().ResolveConfirm(yes);
}

export async function resolveEditText(text: string, cancelled: boolean): Promise<void> {
  if (app()) return app().ResolveEditText(text, cancelled);
}

export async function resolveEditDocument(metadata: Record<string, string>, description: string, cancelled: boolean): Promise<void> {
  if (app()) return app().ResolveEditDocument(metadata, description, cancelled);
}

export async function resolvePromptText(text: string, cancelled: boolean): Promise<void> {
  if (app()) return app().ResolvePromptText(text, cancelled);
}

export async function resolveReviewDiff(index: number): Promise<void> {
  if (app()) return app().ResolveReviewDiff(index);
}

// ── Misc ──

export async function openInBrowser(url: string): Promise<void> {
  if (app()) { app().OpenInBrowser(url); return; }
}
