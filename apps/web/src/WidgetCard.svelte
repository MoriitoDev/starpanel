<script lang="ts">
  import { untrack } from "svelte";
  import type { PluginInfo, Widget, WidgetContext } from "./types";

  interface Props {
    widget: Widget;
    plugin: PluginInfo;
  }

  let { widget, plugin }: Props = $props();

  let host = $state<HTMLDivElement>();
  let error = $state<string | null>(null);
  let cleanupFn: (() => void) | undefined;
  let run = 0;

  function specFor(p: PluginInfo, w: Widget) {
    return (w.widget ? p.widgets.find((x) => x.id === w.widget) : undefined) ?? p.widgets[0];
  }

  // Tracked inputs are primitives only, so the 10s dashboard re-sync does
  // not reset widgets whose content did not change. Everything read inside
  // untrack is captured at mount.
  $effect(() => {
    [
      widget.enabled,
      widget.widget ?? null,
      widget.pollSeconds,
      JSON.stringify(widget.config ?? {}),
      plugin.name
    ];
    const token = ++run;
    cleanupFn?.();
    cleanupFn = undefined;
    error = null;

    untrack(() => {
      const spec = specFor(plugin, widget);
      if (!spec) {
        error = `Plugin "${plugin.name}" provides no widget`;
        return;
      }
      import(/* @vite-ignore */ spec.module)
        .then((mod) => {
          if (token !== run) return;
          if (typeof mod.default !== "function") {
            throw new Error("widget module must export a default render function");
          }
          const target = document.createElement("div");
          host?.replaceChildren(target);
          const ctx: WidgetContext = {
            config: widget.config ?? {},
            pollSeconds: widget.pollSeconds,
            fetch: (path: string, init?: RequestInit) =>
              fetch(`/api/v1/plugins/${plugin.name}/proxy/${path}`, init)
          };
          const result = mod.default(target, ctx);
          if (typeof result === "function") {
            cleanupFn = result;
          }
        })
        .catch((err: unknown) => {
          if (token === run) {
            error = err instanceof Error ? err.message : String(err);
          }
        });
    });

    return () => {
      cleanupFn?.();
      cleanupFn = undefined;
    };
  });
</script>

<div bind:this={host}></div>
{#if error}
  <p class="text-meta text-danger">Widget failed: {error}</p>
{/if}
