/** Mirrors core.WorkItem from the Go domain model. */
export interface WorkItem {
  id: string;
  type: string;
  summary: string;
  status: string;
  parentId: string;
  description: string;
  fields: Record<string, unknown>;
  displayFields: Record<string, unknown>;
  children: WorkItem[];
  comments: Comment[];
  /** Depth in the tree (0 = root). Set during flattening, not from backend. */
  depth?: number;
}

export interface Comment {
  author: string;
  created: string;
  body: string;
}

/** Mirrors core.Workspace. */
export interface Workspace {
  slug: string;
  name: string;
  provider: string;
  baseUrl: string;
  types: TypeConfig[];
  statuses: string[];
  filters: Record<string, string>;
}

export interface TypeConfig {
  id: number;
  name: string;
  order: number;
  color: string;
  hasChildren: boolean;
  template: string;
}

/** Mirrors core.FieldDef. */
export interface FieldDef {
  key: string;
  label: string;
  type: "string" | "enum" | "string_array" | "bool";
  enum: string[];
  visibility: "default" | "extended" | "readonly";
  topLevel: boolean;
}

/** Diff entry for apply review. */
export interface FieldDiff {
  field: string;
  old: string;
  new: string;
}

/** Popup mode discriminator. */
export type PopupMode =
  | { kind: "select"; title: string; options: string[]; onResult: (index: number | null) => void }
  | { kind: "input"; title: string; placeholder: string; onResult: (text: string | null) => void }
  | { kind: "editor"; title: string; content: string; schema?: object; onResult: (result: { action: string; content: string } | null) => void }
  | { kind: "reviewdiff"; title: string; changes: FieldDiff[]; options: string[]; onResult: (index: number | null) => void }
  | { kind: "form"; title: string; metadata: Record<string, string>; description: string; fields: FieldDef[]; statuses: string[]; types: string[]; onResult: (result: { metadata: Record<string, string>; description: string } | null) => void };

export interface Toast {
  message: string;
  type: "success" | "error" | "info";
  id: number;
}
