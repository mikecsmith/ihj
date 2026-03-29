<script lang="ts">
  import { untrack } from "svelte";
  import { basicSetup } from "codemirror";
  import { EditorView, ViewPlugin, Decoration, type DecorationSet, keymap, type ViewUpdate } from "@codemirror/view";
  import { EditorState, type Range } from "@codemirror/state";
  import { indentWithTab } from "@codemirror/commands";
  import { yaml as yamlLang } from "@codemirror/lang-yaml";
  import { linter, type Diagnostic } from "@codemirror/lint";
  import { autocompletion, type CompletionContext, type CompletionResult } from "@codemirror/autocomplete";
  import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
  import { tags } from "@lezer/highlight";
  import Ajv from "ajv";
  import { parse as parseYaml } from "yaml";

  interface Props {
    content: string;
    schema?: object;
    onSave: (yaml: string) => void;
    onCancel: () => void;
  }
  let { content, schema, onSave, onCancel }: Props = $props();

  let editorContainer: HTMLDivElement | undefined = $state();
  let currentContent = $state(untrack(() => content));

  /** Tokyo Night theme for CodeMirror. */
  const editorTheme = EditorView.theme({
    "&": {
      backgroundColor: "#1a1b26",
      color: "#c0caf5",
      fontSize: "13px",
      fontFamily: "var(--font-mono)",
      height: "100%",
      overflow: "hidden",
    },
    ".cm-scroller": { overflow: "auto" },
    ".cm-content": { caretColor: "#7aa2f7", overflowWrap: "anywhere", wordBreak: "break-all" },
    ".cm-line": { overflowWrap: "anywhere", wordBreak: "break-all" },
    ".cm-cursor": { borderLeftColor: "#7aa2f7" },
    ".cm-activeLine": { backgroundColor: "#292e42" },
    ".cm-selectionBackground, ::selection": { backgroundColor: "#283457 !important" },
    ".cm-gutters": {
      backgroundColor: "#16161e",
      color: "#565f89",
      borderRight: "1px solid #3b4261",
    },
    ".cm-activeLineGutter": { backgroundColor: "#292e42" },
    ".cm-tooltip": {
      backgroundColor: "#24283b",
      border: "1px solid #3b4261",
      color: "#c0caf5",
    },
    ".cm-tooltip-autocomplete ul li[aria-selected]": {
      backgroundColor: "#283457",
    },
    ".cm-diagnostic-error": { borderLeftColor: "#f7768e" },
    ".cm-diagnostic-warning": { borderLeftColor: "#e0af68" },
    // Markdown highlighting inside description block scalars.
    ".cm-md-bold": { fontWeight: "bold", color: "#c0caf5" },
    ".cm-md-italic": { fontStyle: "italic", color: "#c0caf5" },
    ".cm-md-bold-italic": { fontWeight: "bold", fontStyle: "italic", color: "#c0caf5" },
    ".cm-md-code": { color: "#ff9e64", backgroundColor: "#292e42", borderRadius: "2px" },
    ".cm-md-heading": { color: "#7aa2f7", fontWeight: "bold" },
    ".cm-md-list-marker": { color: "#bb9af7" },
    ".cm-md-link": { color: "#2ac3de", textDecoration: "underline" },
    ".cm-md-link-url": { color: "#565f89", textDecoration: "none" },
    ".cm-md-code-fence": { color: "#565f89" },
    ".cm-md-code-block": { color: "#ff9e64" },
  });

  /** Tokyo Night syntax highlighting. */
  const tokyoNightHighlight = HighlightStyle.define([
    { tag: tags.keyword, color: "#bb9af7" },
    { tag: tags.operator, color: "#89ddff" },
    { tag: tags.string, color: "#9ece6a" },
    { tag: tags.number, color: "#ff9e64" },
    { tag: tags.bool, color: "#ff9e64" },
    { tag: tags.null, color: "#ff9e64" },
    { tag: tags.comment, color: "#565f89", fontStyle: "italic" },
    { tag: tags.propertyName, color: "#7dcfff" },
    { tag: tags.definition(tags.propertyName), color: "#7dcfff" },
    { tag: tags.tagName, color: "#f7768e" },
    { tag: tags.attributeName, color: "#bb9af7" },
    { tag: tags.variableName, color: "#c0caf5" },
    { tag: tags.typeName, color: "#2ac3de" },
    { tag: tags.meta, color: "#565f89" },
    { tag: tags.atom, color: "#ff9e64" },
    { tag: tags.punctuation, color: "#a9b1d6" },
  ]);

  /** YAML lint source using ajv for JSON Schema validation. */
  function createYamlLinter(jsonSchema: object) {
    const ajv = new Ajv({ allErrors: true, verbose: true });
    let validate: ReturnType<typeof ajv.compile>;
    try {
      validate = ajv.compile(jsonSchema);
    } catch {
      return () => [] as Diagnostic[];
    }

    return (view: EditorView): Diagnostic[] => {
      const text = view.state.doc.toString();
      if (!text.trim()) return [];

      let parsed: unknown;
      try {
        parsed = parseYaml(text);
      } catch (e: any) {
        const line = e.linePos?.[0]?.line ?? 1;
        const from = view.state.doc.line(Math.min(line, view.state.doc.lines)).from;
        return [{ from, to: from + 1, severity: "error", message: e.message ?? "YAML parse error" }];
      }

      validate(parsed);
      if (!validate.errors) return [];

      return validate.errors.map((err) => {
        // Try to map the JSON path back to a line.
        const path = err.instancePath || "";
        const lineNum = findLineForPath(text, path);
        const from = view.state.doc.line(Math.min(lineNum, view.state.doc.lines)).from;
        const to = view.state.doc.line(Math.min(lineNum, view.state.doc.lines)).to;
        return {
          from,
          to,
          severity: "warning" as const,
          message: `${path}: ${err.message ?? "schema error"}`,
        };
      });
    };
  }

  /** Best-effort mapping from a JSON pointer path to a YAML line number. */
  function findLineForPath(text: string, path: string): number {
    if (!path) return 1;
    const parts = path.split("/").filter(Boolean);
    const lines = text.split("\n");

    // Walk the path segments and find matching keys in the YAML text.
    let lineIdx = 0;
    for (const part of parts) {
      const isIndex = /^\d+$/.test(part);
      let count = 0;
      for (let i = lineIdx; i < lines.length; i++) {
        const trimmed = lines[i].trimStart();
        if (isIndex) {
          if (trimmed.startsWith("- ")) {
            if (count === parseInt(part)) { lineIdx = i; break; }
            count++;
          }
        } else {
          if (trimmed.startsWith(part + ":")) { lineIdx = i; break; }
        }
      }
    }
    return lineIdx + 1; // 1-based
  }

  /** Autocomplete source that offers enum values from the schema. */
  function createSchemaCompleter(jsonSchema: object): (ctx: CompletionContext) => CompletionResult | null {
    // Collect all enum values keyed by property name.
    const enumMap = new Map<string, string[]>();
    function walkProps(props: Record<string, any>) {
      for (const [key, def] of Object.entries(props)) {
        if (def.enum) enumMap.set(key, def.enum);
      }
    }

    const s = jsonSchema as any;
    if (s.properties) walkProps(s.properties);
    if (s.$defs?.item?.properties) walkProps(s.$defs.item.properties);

    return (ctx: CompletionContext): CompletionResult | null => {
      // Check if cursor is after "key: " on the current line.
      const line = ctx.state.doc.lineAt(ctx.pos);
      const textBefore = line.text.slice(0, ctx.pos - line.from);
      const match = textBefore.match(/^\s*(\w+):\s*(\S*)$/);
      if (!match) return null;

      const key = match[1];
      const partial = match[2];
      const enums = enumMap.get(key);
      if (!enums) return null;

      const from = ctx.pos - partial.length;
      return {
        from,
        options: enums
          .filter((v) => v.toLowerCase().startsWith(partial.toLowerCase()))
          .map((v) => ({ label: v, type: "enum" })),
      };
    };
  }

  // ── Markdown highlighting inside YAML description block scalars ──

  interface DescBlock {
    /** Absolute doc offset of the first content line. */
    from: number;
    /** Absolute doc offset past the last content line. */
    to: number;
    /** Number of whitespace chars to strip from each line (key indent + 2). */
    baseIndent: number;
  }

  /** Find all `description: |` block scalar regions in the document.
   *  Handles both `description: |` and `- description: |` (list item).
   *  For `- description:`, the `- ` prefix adds 2 to the effective key indent. */
  function findDescriptionBlocks(state: EditorState): DescBlock[] {
    const blocks: DescBlock[] = [];
    const doc = state.doc;

    for (let i = 1; i <= doc.lines; i++) {
      const line = doc.line(i);
      // Match both "  description: |" and "  - description: |"
      // Also handle chomping indicators: |, |-, |+
      const match = line.text.match(/^(\s*)(?:-\s+)?description:\s*\|[+-]?\s*$/);
      if (!match) continue;

      // The content indent is always relative to where the key text starts.
      // For "  description:", keyCol = 2, content at 2 + 2 = 4.
      // For "  - description:", keyCol = 4 (after "- "), content at 4 + 2 = 6.
      const leadingSpaces = match[1].length;
      const hasDash = line.text.trimStart().startsWith("-");
      const keyCol = hasDash ? leadingSpaces + 2 : leadingSpaces;
      const contentIndent = keyCol + 2;
      const blockFrom = i + 1;
      let blockTo = i;

      // Collect lines that belong to this block scalar: every subsequent line
      // that is either blank or indented deeper than the key.
      for (let j = blockFrom; j <= doc.lines; j++) {
        const cl = doc.line(j);
        if (cl.text.length === 0 || cl.text.match(/^\s+$/) ) {
          // Blank/whitespace-only lines are part of the block.
          blockTo = j;
          continue;
        }
        const lineIndent = cl.text.match(/^(\s*)/)![1].length;
        if (lineIndent >= contentIndent) {
          blockTo = j;
        } else {
          break;
        }
      }

      if (blockTo >= blockFrom) {
        blocks.push({
          from: doc.line(blockFrom).from,
          to: doc.line(blockTo).to,
          baseIndent: contentIndent,
        });
      }
    }
    return blocks;
  }

  const mdBold = Decoration.mark({ class: "cm-md-bold" });
  const mdItalic = Decoration.mark({ class: "cm-md-italic" });
  const mdBoldItalic = Decoration.mark({ class: "cm-md-bold-italic" });
  const mdCode = Decoration.mark({ class: "cm-md-code" });
  const mdHeading = Decoration.mark({ class: "cm-md-heading" });
  const mdListMarker = Decoration.mark({ class: "cm-md-list-marker" });
  const mdLink = Decoration.mark({ class: "cm-md-link" });
  const mdLinkUrl = Decoration.mark({ class: "cm-md-link-url" });
  const mdCodeFence = Decoration.mark({ class: "cm-md-code-fence" });
  const mdCodeBlockLine = Decoration.mark({ class: "cm-md-code-block" });

  /** Tokenize a single line of markdown and return decorations at absolute offsets. */
  function tokenizeMdLine(
    lineText: string,
    lineFrom: number,
    baseIndent: number,
  ): Range<Decoration>[] {
    const decos: Range<Decoration>[] = [];
    // Strip the YAML base indent to get the markdown content.
    const stripped = lineText.length > baseIndent ? lineText.slice(baseIndent) : lineText.trimStart();
    const contentStart = lineFrom + (lineText.length - stripped.length);

    // Headings: lines starting with # (after indent strip).
    const headingMatch = stripped.match(/^(#{1,6})\s+/);
    if (headingMatch) {
      decos.push(mdHeading.range(contentStart, lineFrom + lineText.length));
      return decos;
    }

    // List markers: `- ` or `* ` or `1. ` at start of stripped content.
    const listMatch = stripped.match(/^(\s*)([-*]|\d+\.)\s/);
    if (listMatch) {
      const markerStart = contentStart + listMatch[1].length;
      const markerEnd = markerStart + listMatch[2].length;
      decos.push(mdListMarker.range(markerStart, markerEnd));
    }

    // Inline patterns — scan the full stripped content.
    const patterns: { re: RegExp; deco: Decoration; group?: number; extra?: (m: RegExpExecArray, off: number) => void }[] = [
      // Bold+italic (must come before bold/italic)
      { re: /(\*{3}|_{3})(?!\s)(.+?)(?<!\s)\1/g, deco: mdBoldItalic },
      // Bold
      { re: /(\*{2}|_{2})(?!\s)(.+?)(?<!\s)\1/g, deco: mdBold },
      // Italic (but not inside bold markers we already matched — handled by ordering)
      { re: /(?<!\w)([*_])(?!\s)(.+?)(?<!\s)\1(?!\w)/g, deco: mdItalic },
      // Inline code
      { re: /`([^`]+)`/g, deco: mdCode },
      // Links: [text](url)
      {
        re: /\[([^\]]+)\]\(([^)]+)\)/g,
        deco: mdLink,
        extra: (m, off) => {
          // Decorate the URL portion differently.
          const urlStart = off + m[0].indexOf("](") + 2;
          const urlEnd = off + m[0].length - 1;
          decos.push(mdLinkUrl.range(urlStart, urlEnd));
        },
      },
    ];

    for (const { re, deco, extra } of patterns) {
      re.lastIndex = 0;
      let m: RegExpExecArray | null;
      while ((m = re.exec(stripped)) !== null) {
        const from = contentStart + m.index;
        const to = from + m[0].length;
        decos.push(deco.range(from, to));
        if (extra) extra(m, contentStart);
      }
    }

    return decos;
  }

  /** Build all markdown decorations for all description blocks. */
  function buildMdDecorations(state: EditorState): DecorationSet {
    const blocks = findDescriptionBlocks(state);
    if (blocks.length === 0) return Decoration.none;

    const allDecos: Range<Decoration>[] = [];

    for (const block of blocks) {
      const doc = state.doc;
      const startLine = doc.lineAt(block.from).number;
      const endLine = doc.lineAt(block.to).number;

      // First pass: find fenced code block regions so we can skip inline
      // markdown tokenization inside them.
      const codeBlockLines = new Set<number>();
      let fenceOpen = -1;
      for (let i = startLine; i <= endLine; i++) {
        const line = doc.line(i);
        const stripped = line.text.length > block.baseIndent
          ? line.text.slice(block.baseIndent)
          : line.text.trimStart();

        if (stripped.match(/^```/)) {
          if (fenceOpen === -1) {
            // Opening fence.
            fenceOpen = i;
            codeBlockLines.add(i);
            allDecos.push(mdCodeFence.range(line.from, line.to));
          } else {
            // Closing fence.
            codeBlockLines.add(i);
            allDecos.push(mdCodeFence.range(line.from, line.to));
            fenceOpen = -1;
          }
        } else if (fenceOpen !== -1) {
          // Inside a fenced code block.
          codeBlockLines.add(i);
          if (line.text.trim() !== "") {
            allDecos.push(mdCodeBlockLine.range(line.from, line.to));
          }
        }
      }

      // Second pass: tokenize markdown on non-code-block lines.
      for (let i = startLine; i <= endLine; i++) {
        if (codeBlockLines.has(i)) continue;
        const line = doc.line(i);
        if (line.text.trim() === "") continue;
        const lineDecos = tokenizeMdLine(line.text, line.from, block.baseIndent);
        allDecos.push(...lineDecos);
      }
    }

    // Decorations must be sorted by from position.
    allDecos.sort((a, b) => a.from - b.from);
    return Decoration.set(allDecos);
  }

  const markdownHighlighter = ViewPlugin.fromClass(
    class {
      decorations: DecorationSet;
      constructor(view: EditorView) {
        this.decorations = buildMdDecorations(view.state);
      }
      update(update: ViewUpdate) {
        if (update.docChanged) {
          this.decorations = buildMdDecorations(update.state);
        }
      }
    },
    { decorations: (v) => v.decorations },
  );

  $effect(() => {
    if (!editorContainer) return;

    const extensions = [
      basicSetup,
      yamlLang(),
      editorTheme,
      syntaxHighlighting(tokyoNightHighlight),
      EditorView.lineWrapping,
      keymap.of([
        indentWithTab,
        {
          key: "Mod-s",
          run: () => { onSave(currentContent); return true; },
        },
        {
          key: "Escape",
          run: () => { onCancel(); return true; },
        },
      ]),
      EditorView.updateListener.of((update: ViewUpdate) => {
        if (update.docChanged) {
          currentContent = update.state.doc.toString();
        }
      }),
      markdownHighlighter,
    ];

    if (schema) {
      extensions.push(linter(createYamlLinter(schema)));
      extensions.push(autocompletion({ override: [createSchemaCompleter(schema)] }));
    }

    const view = new EditorView({
      state: EditorState.create({ doc: content, extensions }),
      parent: editorContainer,
    });

    return () => view.destroy();
  });
</script>

<div class="bulk-editor">
  <div class="bulk-editor__toolbar">
    <span class="bulk-editor__title">Bulk Edit — YAML Manifest</span>
    <div class="bulk-editor__actions">
      <button class="bulk-editor__btn" onclick={onCancel}>Cancel</button>
      <button class="bulk-editor__btn bulk-editor__btn--primary" onclick={() => onSave(currentContent)}>Apply</button>
    </div>
  </div>
  <div class="bulk-editor__cm" bind:this={editorContainer}></div>
  <div class="bulk-editor__hint">Ctrl/Cmd+S Apply &middot; Esc Cancel &middot; Schema validation active</div>
</div>

<style>
  .bulk-editor { display: flex; flex-direction: column; height: 100%; width: 100%; min-height: 0; overflow: hidden; }
  .bulk-editor__toolbar { display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; border-bottom: 1px solid var(--border-default); flex-shrink: 0; }
  .bulk-editor__title { font-size: 12px; font-weight: 600; color: var(--accent-blue); text-transform: uppercase; letter-spacing: 0.5px; }
  .bulk-editor__actions { display: flex; gap: 6px; }
  .bulk-editor__btn { padding: 4px 12px; border-radius: var(--radius-sm); font-family: var(--font-mono); font-size: 11px; font-weight: 600; cursor: pointer; border: 1px solid var(--border-default); background: var(--bg-tertiary); color: var(--text-secondary); transition: all var(--transition-fast); }
  .bulk-editor__btn:hover { background: var(--bg-hover); color: var(--text-primary); }
  .bulk-editor__btn--primary { background: var(--accent-blue); color: #fff; border-color: var(--accent-blue); }
  .bulk-editor__btn--primary:hover { opacity: 0.9; }
  .bulk-editor__cm { flex: 1; min-height: 0; overflow: hidden; }
  .bulk-editor__cm :global(.cm-editor) { height: 100%; overflow: hidden; }
  .bulk-editor__cm :global(.cm-scroller) { overflow: auto !important; }
  .bulk-editor__hint { font-size: 10px; color: var(--text-hint); padding: 4px 12px; border-top: 1px solid var(--border-default); flex-shrink: 0; }
</style>
