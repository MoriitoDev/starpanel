<script lang="ts">
  import { onMount } from "svelte";
  import { fetchDashboard, fetchPlugins, saveDashboard } from "./api";
  import StarMark from "./lib/StarMark.svelte";
  import StatusDot from "./lib/StatusDot.svelte";
  import WidgetCard from "./WidgetCard.svelte";
  import type { Dashboard, PluginError, PluginInfo, Widget, WidgetSize } from "./types";

  type Health = "checking" | "alive" | "degraded" | "offline";
  type SaveState = "saved" | "saving" | "error";

  /** How often the shell re-reads the Dashboard; the status popover shows it. */
  const DASHBOARD_POLL_SECONDS = 10;

  let health = $state<Health>("checking");
  let dashboard = $state<Dashboard | null>(null);
  let plugins = $state<PluginInfo[]>([]);
  let pluginErrors = $state<PluginError[]>([]);
  let saveState = $state<SaveState>("saved");
  let saveError = $state<string | null>(null);
  let addSelection = $state("");
  let editing = $state(false);

  function pluginFor(widget: Widget): PluginInfo | null {
    return plugins.find((p) => p.name === widget.plugin) ?? null;
  }

  function effectiveWidgetId(widget: Widget): string {
    return widget.widget ?? pluginFor(widget)?.widgets[0]?.id ?? "";
  }

  function titleFor(widget: Widget): string {
    const plugin = pluginFor(widget);
    if (plugin) {
      const spec = plugin.widgets.find((x) => x.id === effectiveWidgetId(widget));
      if (spec?.title) return spec.title;
    }
    const label = widget.config?.label;
    return typeof label === "string" ? label : widget.id;
  }

  function spanFor(size: WidgetSize): string {
    if (size === "small") return "md:col-span-3 lg:col-span-4";
    if (size === "large") return "md:col-span-6 lg:col-span-12";
    return "md:col-span-6 lg:col-span-6";
  }

  interface WidgetOption {
    value: string;
    label: string;
    present: boolean;
  }

  function widgetOptions(): WidgetOption[] {
    const options: WidgetOption[] = [];
    for (const plugin of plugins) {
      for (const spec of plugin.widgets) {
        const present = dashboard?.widgets.some(
          (w) => w.plugin === plugin.name && effectiveWidgetId(w) === spec.id
        );
        options.push({
          value: `${plugin.name}|${spec.id}`,
          label: `${plugin.name} · ${spec.title}`,
          present: present ?? false
        });
      }
    }
    return options;
  }

  async function addWidget(): Promise<void> {
    if (!dashboard || !addSelection) return;
    const [pluginName, widgetId] = addSelection.split("|");
    const plugin = plugins.find((p) => p.name === pluginName);
    const spec = plugin?.widgets.find((x) => x.id === widgetId);
    if (!plugin || !spec) return;
    const base = `${pluginName}-${widgetId}`;
    let id = base;
    for (let n = 2; dashboard.widgets.some((w) => w.id === id); n += 1) {
      id = `${base}-${n}`;
    }
    const widgets = [
      ...dashboard.widgets,
      {
        id,
        plugin: pluginName,
        widget: widgetId,
        size: "medium",
        enabled: true,
        pollSeconds: DASHBOARD_POLL_SECONDS
      } as Widget
    ];
    addSelection = "";
    await persist({ ...dashboard, widgets });
  }

  async function removeWidget(widget: Widget): Promise<void> {
    if (!dashboard) return;
    const widgets = dashboard.widgets.filter((w) => w.id !== widget.id);
    await persist({ ...dashboard, widgets });
  }

  async function persist(next: Dashboard): Promise<void> {
    dashboard = next;
    saveState = "saving";
    saveError = null;
    try {
      dashboard = await saveDashboard(next);
      saveState = "saved";
    } catch (err) {
      saveState = "error";
      saveError = err instanceof Error ? err.message : String(err);
    }
  }

  function move(index: number, delta: number): void {
    if (!dashboard) return;
    const widgets = [...dashboard.widgets];
    const target = index + delta;
    if (target < 0 || target >= widgets.length) return;
    [widgets[index], widgets[target]] = [widgets[target], widgets[index]];
    void persist({ ...dashboard, widgets });
  }

  function toggle(widget: Widget): void {
    if (!dashboard) return;
    const widgets = dashboard.widgets.map((w) =>
      w.id === widget.id ? { ...w, enabled: !w.enabled } : w
    );
    void persist({ ...dashboard, widgets });
  }

  async function refreshPlugins(): Promise<void> {
    try {
      const list = await fetchPlugins();
      plugins = list.plugins;
      pluginErrors = list.errors;
    } catch {
      // Keep the last listing; the dashboard itself must not break.
    }
  }

  onMount(() => {
    void (async () => {
      try {
        const res = await fetch("/api/v1/health");
        const body = await res.json();
        health = body.status === "ok" ? "alive" : "degraded";
      } catch {
        health = "offline";
        return;
      }
      try {
        dashboard = await fetchDashboard();
      } catch {
        health = "offline";
      }
      await refreshPlugins();
    })();

    // 10s REST poll, the widget default from the spec. Also picks up
    // newly dropped plugin folders.
    const timer = setInterval(() => {
      if (saveState === "saving") return;
      fetchDashboard()
        .then((fresh) => (dashboard = fresh))
        .catch(() => {});
      void refreshPlugins();
    }, DASHBOARD_POLL_SECONDS * 1000);

    return () => {
      clearInterval(timer);
    };
  });
</script>

<div class="min-h-dvh bg-canvas font-sans text-body">
  <div class="mx-auto flex max-w-[1200px] flex-col px-4 py-6 sm:px-6">
    {#if dashboard}
      <div class="flex items-center justify-between gap-4">
        <StatusDot
          health={health}
          save={saveState}
          saveError={saveError}
          pollSeconds={DASHBOARD_POLL_SECONDS}
        />
        <div class="flex items-center gap-2">
          <button
            type="button"
            class={editing ? "btn btn-primary" : "btn btn-secondary"}
            aria-pressed={editing}
            onclick={() => (editing = !editing)}
            >{editing ? "Done" : "Edit"}</button
          >
        </div>
      </div>
    {/if}

    <header class="flex items-center justify-center gap-3 py-10">
      <StarMark class="h-8 w-8 text-accent" />
      <h1 class="text-display text-ink">Star Panel</h1>
    </header>

    <main class="flex flex-col gap-8">
      {#if !dashboard}
        <p class="text-center text-meta text-mute">
          {health === "offline"
            ? "Waiting for the API on :8080…"
            : "Loading the Dashboard…"}
        </p>
      {:else}
        {#if editing && plugins.length > 0}
          <div class="flex flex-wrap items-center gap-2">
            <select class="control" bind:value={addSelection} aria-label="Widget to add">
              <option value="" disabled>Add widget…</option>
              {#each widgetOptions() as option (option.value)}
                <option value={option.value} disabled={option.present}>
                  {option.label}{option.present ? " (on the panel)" : ""}
                </option>
              {/each}
            </select>
            <button
              type="button"
              class="btn btn-primary"
              disabled={!addSelection}
              onclick={() => addWidget()}>Add</button
            >
          </div>
        {/if}

        {#if dashboard.widgets.length === 0}
          <div class="py-14 text-center">
            <p class="text-display text-ink">No widgets yet</p>
            <p class="mt-2 text-base text-body">
              {editing ? "Pick one above to add it." : "Enter edit mode to add one."}
            </p>
          </div>
        {:else}
          <ul class="grid grid-cols-1 gap-4 md:grid-cols-6 lg:grid-cols-12 lg:gap-6">
            {#each dashboard.widgets as w, i (w.id)}
              <li class={spanFor(w.size)}>
                <section
                  class="flex h-full flex-col rounded-md border border-border bg-surface p-5"
                  aria-labelledby={`widget-${w.id}`}
                >
                  <div class="flex items-start justify-between gap-3">
                    <h2 id={`widget-${w.id}`} class="text-subheading text-ink">{titleFor(w)}</h2>
                    {#if editing}
                      <div class="flex items-center gap-1">
                        <label class="flex items-center gap-2 px-2 text-meta text-mute">
                          <input
                            type="checkbox"
                            class="h-4 w-4 accent-accent"
                            checked={w.enabled}
                            onchange={() => toggle(w)}
                          />
                          enabled
                        </label>
                        <button
                          type="button"
                          class="btn btn-ghost px-2 text-meta"
                          aria-label="Remove {titleFor(w)}"
                          onclick={() => removeWidget(w)}>Remove</button
                        >
                      </div>
                    {/if}
                  </div>

                  <div class="mt-4 flex-1">
                    {#if !w.enabled}
                      <p class="text-meta text-mute">Disabled</p>
                    {:else if pluginFor(w)}
                      <WidgetCard widget={w} plugin={pluginFor(w)!} />
                    {:else}
                      <p class="text-meta text-danger">
                        Plugin "{w.plugin}" is missing — drop its folder into the plugins
                        directory.
                      </p>
                    {/if}
                  </div>

                  {#if editing}
                    <div class="mt-4 flex items-center gap-1">
                      <button
                        type="button"
                        class="btn btn-secondary px-3"
                        disabled={i === 0}
                        aria-label="Move {titleFor(w)} up"
                        onclick={() => move(i, -1)}>↑</button
                      >
                      <button
                        type="button"
                        class="btn btn-secondary px-3"
                        disabled={i === dashboard.widgets.length - 1}
                        aria-label="Move {titleFor(w)} down"
                        onclick={() => move(i, 1)}>↓</button
                      >
                    </div>
                  {:else}
                    <p class="mt-4 text-meta text-mute">poll {w.pollSeconds}s</p>
                  {/if}
                </section>
              </li>
            {/each}
          </ul>
        {/if}

        {#if saveState === "error"}
          <section
            class="rounded-md border border-danger bg-surface p-5"
            aria-labelledby="save-error"
          >
            <h2 id="save-error" class="text-subheading text-ink">Could not save the Dashboard</h2>
            <p class="mt-1 text-base text-body">
              {saveError ?? "The API rejected the document."}
            </p>
          </section>
        {/if}

        {#if pluginErrors.length > 0}
          <section
            class="rounded-md border border-danger bg-surface p-5"
            aria-labelledby="plugin-errors"
          >
            <h2 id="plugin-errors" class="text-subheading text-ink">
              Rejected plugin folders
            </h2>
            <ul class="mt-2 space-y-1">
              {#each pluginErrors as e (e.folder)}
                <li class="text-base text-body">
                  <span class="text-ink">{e.folder}</span>: {e.error}
                </li>
              {/each}
            </ul>
          </section>
        {/if}
      {/if}
    </main>
  </div>
</div>
