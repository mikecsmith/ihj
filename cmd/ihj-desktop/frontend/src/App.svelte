<script lang="ts">
  import { onMount } from "svelte";
  import { get } from "svelte/store";

  import TitleBar from "./components/TitleBar.svelte";
  import SearchBar from "./components/SearchBar.svelte";
  import WorkItemList from "./components/WorkItemList.svelte";
  import DetailPane from "./components/DetailPane.svelte";
  import ActionBar from "./components/ActionBar.svelte";
  import Popup from "./components/Popup.svelte";
  import Toast from "./components/Toast.svelte";

  import { workspace, roots, fieldDefs, filter, fetchedAt } from "./lib/stores/items";
  import { popup, filtered, selectedItem, showToast, moveCursor, setCursor } from "./lib/stores/ui";
  import { setupKeyboard, type ActionMap } from "./actions/keyboard";
  import {
    getWorkspace,
    searchItems,
    getFieldDefs,
    openInBrowser,
    exportManifest,
    getManifestSchema,
    runAssign,
    runTransition,
    runCreate,
    runEdit,
    runComment,
    runBranch,
    runExtract,
    runApplyManifest,
  } from "./lib/bindings";
  import { setupUIBridge } from "./lib/uiBridge";

  let searchBarRef: SearchBar | undefined = $state();

  async function refresh(noCache = true, silent = false) {
    try {
      const f = get(filter);
      const data = await searchItems(f, noCache);
      roots.set(data);
      fetchedAt.set(new Date());
      if (!silent) showToast("Refreshed", "success");
    } catch (e: any) {
      showToast(e.message ?? "Refresh failed", "error");
    }
  }

  function handleAction(action: string) {
    const actions: ActionMap = {
      refresh,

      filter: () => {
        const ws = get(workspace);
        if (!ws) return;
        const filterNames = Object.keys(ws.filters);
        popup.set({
          kind: "select",
          title: "Select Filter",
          options: filterNames,
          onResult: (idx) => {
            if (idx !== null && idx >= 0) {
              filter.set(filterNames[idx]);
              refresh();
            }
          },
        });
      },

      assign: async () => {
        const item = get(selectedItem);
        if (!item) return;
        try {
          await runAssign(item.id);
          await refresh(true, true);
        } catch (e: any) {
          showToast(e.message ?? "Assign failed", "error");
        }
      },

      transition: async () => {
        const item = get(selectedItem);
        if (!item) return;
        try {
          await runTransition(item.id);
          await refresh(true, true);
        } catch (e: any) {
          showToast(e.message ?? "Transition failed", "error");
        }
      },

      open: () => {
        const ws = get(workspace);
        const item = get(selectedItem);
        if (!ws || !item) return;
        openInBrowser(`${ws.baseUrl}/browse/${item.id}`);
      },

      edit: async () => {
        const item = get(selectedItem);
        if (!item) return;
        try {
          await runEdit(item.id);
          await refresh(true, true);
        } catch (e: any) {
          showToast(e.message ?? "Edit failed", "error");
        }
      },

      comment: async () => {
        const item = get(selectedItem);
        if (!item) return;
        try {
          await runComment(item.id);
          await refresh(true, true);
        } catch (e: any) {
          showToast(e.message ?? "Comment failed", "error");
        }
      },

      branch: async () => {
        const item = get(selectedItem);
        if (!item) return;
        try {
          await runBranch(item.id);
        } catch (e: any) {
          showToast(e.message ?? "Branch failed", "error");
        }
      },

      extract: async () => {
        const item = get(selectedItem);
        if (!item) return;
        try {
          await runExtract(item.id);
        } catch (e: any) {
          showToast(e.message ?? "Extract failed", "error");
        }
      },

      create: async () => {
        try {
          await runCreate();
          await refresh(true, true);
        } catch (e: any) {
          showToast(e.message ?? "Create failed", "error");
        }
      },

      bulkEdit: async () => {
        try {
          const f = get(filter);
          const [yamlContent, jsonSchema] = await Promise.all([
            exportManifest(f, false),
            getManifestSchema(),
          ]);
          popup.set({
            kind: "editor",
            title: "Bulk Edit",
            content: yamlContent,
            schema: jsonSchema,
            onResult: async (result) => {
              if (result && result.action === "save") {
                try {
                  await runApplyManifest(result.content);
                  await refresh(true, true);
                } catch (e: any) {
                  showToast(e.message ?? "Apply failed", "error");
                }
              }
            },
          });
        } catch (e: any) {
          showToast(e.message ?? "Export failed", "error");
        }
      },
    };

    const fn = actions[action as keyof ActionMap];
    if (fn) fn();
  }

  function handleNavigate(id: string) {
    const list = get(filtered);
    const idx = list.findIndex((item) => item.id === id);
    if (idx >= 0) setCursor(idx);
  }

  function handleListKeydown(e: KeyboardEvent) {
    // Don't handle arrow keys when popup is open.
    if (get(popup)) return;

    if (e.key === "ArrowUp") { e.preventDefault(); moveCursor(-1); }
    else if (e.key === "ArrowDown") { e.preventDefault(); moveCursor(1); }
    else if (e.key === "PageUp") { e.preventDefault(); moveCursor(-10); }
    else if (e.key === "PageDown") { e.preventDefault(); moveCursor(10); }
    else if (e.key === "Home") { e.preventDefault(); setCursor(0); }
    else if (e.key === "End") { e.preventDefault(); setCursor(get(filtered).length - 1); }
    else if (e.key === "/" || e.key === "f" && e.ctrlKey) {
      e.preventDefault();
      searchBarRef?.focus();
    }
  }

  onMount(() => {
    // Start listening for Go → frontend UI events (popups, toasts, etc.).
    const cleanupBridge = setupUIBridge();

    // Load initial data (fire-and-forget — errors shown as toasts).
    (async () => {
      try {
        const [ws, defs, data] = await Promise.all([
          getWorkspace(),
          getFieldDefs(),
          searchItems("active"),
        ]);
        workspace.set(ws);
        fieldDefs.set(defs);
        roots.set(data);
        fetchedAt.set(new Date());
      } catch (e: any) {
        showToast(e.message ?? "Failed to load data", "error");
      }
    })();

    // Set up keyboard shortcuts.
    const cleanupKeyboard = setupKeyboard({
      refresh,
      filter: () => handleAction("filter"),
      assign: () => handleAction("assign"),
      transition: () => handleAction("transition"),
      open: () => handleAction("open"),
      edit: () => handleAction("edit"),
      comment: () => handleAction("comment"),
      branch: () => handleAction("branch"),
      extract: () => handleAction("extract"),
      create: () => handleAction("create"),
      bulkEdit: () => handleAction("bulkEdit"),
    });

    return () => {
      cleanupBridge();
      cleanupKeyboard();
    };
  });
</script>

<svelte:window on:keydown={handleListKeydown} />

<TitleBar />
<DetailPane onNavigate={handleNavigate} />
<SearchBar bind:this={searchBarRef} />
<WorkItemList />
<ActionBar onAction={handleAction} />
<Popup />
<Toast />
