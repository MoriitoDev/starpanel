<script lang="ts">
  import { onMount } from "svelte";
  import { flip } from "svelte/animate";
  import {
    dragHandle,
    dragHandleZone,
    SHADOW_ITEM_MARKER_PROPERTY_NAME
  } from "svelte-dnd-action";
  import {
    deleteTheme,
    fetchDashboard,
    fetchPlugins,
    fetchThemes,
    importPlugin,
    importTheme,
    pluginArchiveHref,
    saveDashboard,
    themeHref
  } from "./api";
  import StarMark from "./lib/StarMark.svelte";
  import Icon from "./lib/Icon.svelte";
  import {
    columnsForWidth,
    resizeSpan,
    type GridMetrics,
    type ResizeAxis,
    type Span
  } from "./lib/layout";
  import StatusDot from "./lib/StatusDot.svelte";
  import WidgetCard from "./WidgetCard.svelte";
  import type {
    Dashboard,
    PluginError,
    PluginInfo,
    ThemeInfo,
    Widget
  } from "./types";

  type Health = "checking" | "alive" | "degraded" | "offline";
  type SaveState = "saved" | "saving" | "error";
  /** A Widget list during a drag also holds the library's placeholder item. */
  type PlaceholderWidget = Widget & { [SHADOW_ITEM_MARKER_PROPERTY_NAME]?: boolean };

  /** How often the shell re-reads the Dashboard; the status popover shows it. */
  const DASHBOARD_POLL_SECONDS = 10;

  /** The flip the cards make as a drag makes room for one of them (§3), which
      someone who asked for reduced motion does not get. */
  const flipDurationMs = window.matchMedia("(prefers-reduced-motion: reduce)").matches
    ? 0
    : 140;

  /** The new Span an added Widget starts at: half the grid, one row tall. */
  const DEFAULT_SPAN: Span = { w: 6, h: 1 };

  let health = $state<Health>("checking");
  let dashboard = $state<Dashboard | null>(null);
  let plugins = $state<PluginInfo[]>([]);
  let pluginErrors = $state<PluginError[]>([]);
  let saveState = $state<SaveState>("saved");
  let saveError = $state<string | null>(null);
  let addSelection = $state("");
  let editing = $state(false);
  // The default is always on offer, even before the first listing arrives.
  let themes = $state<ThemeInfo[]>([{ name: "Default", slug: "default", present: true }]);
  let themeError = $state<string | null>(null);
  let themeNotice = $state<string | null>(null);
  let fileInput = $state<HTMLInputElement>();
  let pluginFileInput = $state<HTMLInputElement>();
  let pluginError = $state<string | null>(null);
  let pluginNotice = $state<string | null>(null);
  // The import itself succeeded; this is what the Plugin cannot find here.
  let pluginProblem = $state<string | null>(null);

  // Arranging (ADR-0008): the grid the cards are drawn in, the Span a card is
  // being dragged out of, and whether a gesture is in flight — which is what
  // keeps the 10s refresh from replacing the list under the pointer.
  let grid = $state<HTMLElement>();
  let arranging = $state(false);
  let resizing = $state<{ id: string; from: Span; span: Span } | null>(null);
  // The grid the browser is showing: below 640px a card fills the only column
  // there is, so the drag is off and the ↑/↓ controls are the way to move one.
  let viewportWidth = $state(window.innerWidth);
  let canArrange = $derived(editing && columnsForWidth(viewportWidth) > 1);

  let activeTheme = $derived(dashboard?.theme ?? "default");
  let activeThemeName = $derived(
    themes.find((theme) => theme.slug === activeTheme)?.name ?? activeTheme
  );
  let activeThemeIsImported = $derived(activeTheme !== "default");

  // A page load arrives with the active Theme already linked, because the
  // server writes the <link> into the shell it serves. Whatever changes it
  // afterwards — this panel, another tab, a hand-edited document — the link in
  // the head follows, and this is a no-op when it is already right.
  function applyTheme(slug: string): void {
    const link = document.head.querySelector<HTMLLinkElement>("link[data-theme]");
    if (slug === "default") {
      link?.remove();
      return;
    }
    const href = themeHref(slug);
    if (link) {
      if (link.getAttribute("href") !== href) link.setAttribute("href", href);
      return;
    }
    const fresh = document.createElement("link");
    fresh.rel = "stylesheet";
    fresh.setAttribute("data-theme", slug);
    fresh.href = href;
    document.head.append(fresh);
  }

  $effect(() => {
    applyTheme(activeTheme);
  });

  async function refreshThemes(): Promise<void> {
    try {
      themes = (await fetchThemes()).themes;
    } catch {
      // Keep the last listing: the panel still renders with the Theme it has.
    }
  }

  function chooseTheme(slug: string): void {
    if (!dashboard || slug === activeTheme) return;
    void persist({ ...dashboard, theme: slug });
  }

  async function importThemeFile(file: File): Promise<void> {
    themeError = null;
    themeNotice = null;
    try {
      const imported = await importTheme(await file.text());
      themeNotice = `${imported.name} imported.`;
      await refreshThemes();
    } catch (err) {
      themeError = err instanceof Error ? err.message : String(err);
    }
  }

  async function removeTheme(theme: ThemeInfo): Promise<void> {
    if (!dashboard) return;
    if (!window.confirm(`Delete the theme "${theme.name}"? Its file goes with it.`)) return;
    themeError = null;
    themeNotice = null;
    try {
      await deleteTheme(theme.slug);
      // Deleting the active Theme is allowed; the panel returns to its default.
      const next = activeTheme === theme.slug ? { ...dashboard, theme: "default" } : dashboard;
      await persist(next);
      await refreshThemes();
    } catch (err) {
      themeError = err instanceof Error ? err.message : String(err);
    }
  }

  async function importPluginFile(file: File): Promise<void> {
    pluginError = null;
    pluginNotice = null;
    pluginProblem = null;
    try {
      const imported = await importPlugin(file);
      pluginNotice = `${imported.name} ${imported.version} imported.`;
      pluginProblem = imported.problem ?? null;
      await refreshPlugins();
    } catch (err) {
      pluginError = err instanceof Error ? err.message : String(err);
    }
  }

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

  /** The classes a Span becomes; app.css draws the breakpoints (§6). */
  function spanClasses(span: Span): string {
    return `span-w-${span.w} span-h-${span.h}`;
  }

  /** The Span a card is drawn at: the one under the pointer, or the stored one. */
  function spanOf(widget: Widget): Span {
    return resizing?.id === widget.id ? resizing.span : { w: widget.w, h: widget.h };
  }

  /**
   * The drag library marks the place a card will land with a property on a
   * stand-in item, and that item is what the dashed placeholder is drawn for.
   */
  function isPlaceholder(widget: Widget): boolean {
    return Boolean((widget as PlaceholderWidget)[SHADOW_ITEM_MARKER_PROPERTY_NAME]);
  }

  /** The Widgets of a list, with any placeholder left out: what gets stored. */
  function storedWidgets(widgets: Widget[]): Widget[] {
    return widgets.filter((widget) => !isPlaceholder(widget));
  }

  interface WidgetOption {
    value: string;
    label: string;
    present: boolean;
    problem: string;
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
          present: present ?? false,
          problem: plugin.problem ?? ""
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
        w: DEFAULT_SPAN.w,
        h: DEFAULT_SPAN.h,
        enabled: true,
        pollSeconds: DASHBOARD_POLL_SECONDS
      } as Widget
    ];
    addSelection = "";
    await persist({ ...dashboard, widgets });
  }

  /** What the add control says it is about to add, when anything is chosen. */
  function addLabel(): string {
    const option = widgetOptions().find((candidate) => candidate.value === addSelection);
    return option ? `Add ${option.label}` : "Add a Widget";
  }

  // The drag library answers with the reordered list on every `consider` and
  // once more on `finalize`: the gesture is the preview, the PUT is the commit
  // (ADR-0008). Reordering never touches the keys a Widget re-mounts on, so a
  // card keeps polling while it is being moved.
  function handleConsider(event: CustomEvent<{ items: Widget[] }>): void {
    if (!dashboard) return;
    arranging = true;
    dashboard = { ...dashboard, widgets: event.detail.items };
  }

  function handleFinalize(event: CustomEvent<{ items: Widget[] }>): void {
    arranging = false;
    if (!dashboard) return;
    void persist({ ...dashboard, widgets: storedWidgets(event.detail.items) });
  }

  /** What a resize needs to know about the grid: measured where it is drawn. */
  function gridMetrics(): GridMetrics {
    const columns = columnsForWidth(window.innerWidth);
    const style = grid ? getComputedStyle(grid) : null;
    const gapPx = style ? parseFloat(style.columnGap) || 0 : 0;
    const width = grid?.clientWidth ?? 0;
    const rowHeightPx =
      parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--row-height")) || 0;
    return {
      columns,
      gapPx,
      rowHeightPx,
      columnPx: columns > 0 ? (width - gapPx * (columns - 1)) / columns : 0
    };
  }

  /**
   * A card's edge or corner: the card follows the pointer and the Span is saved
   * when the gesture ends. A row is a minimum (DESIGN.md §6), so this can grow
   * a card and never clip one.
   */
  function startResize(event: PointerEvent, widget: Widget, axis: ResizeAxis): void {
    const handle = event.currentTarget as HTMLElement;
    const metrics = gridMetrics();
    const from: Span = { w: widget.w, h: widget.h };
    const originX = event.clientX;
    const originY = event.clientY;

    handle.setPointerCapture(event.pointerId);
    arranging = true;
    resizing = { id: widget.id, from, span: from };

    const move = (moved: PointerEvent) => {
      const next = {
        id: widget.id,
        from,
        span: resizeSpan(
          from,
          { dxPx: moved.clientX - originX, dyPx: moved.clientY - originY },
          metrics,
          axis
        )
      };
      resizing = next;
    };
    const end = () => {
      handle.removeEventListener("pointermove", move);
      handle.removeEventListener("pointerup", end);
      handle.removeEventListener("pointercancel", end);
      const span = resizing?.id === widget.id ? resizing.span : from;
      resizing = null;
      arranging = false;
      if (!dashboard || (span.w === from.w && span.h === from.h)) return;
      const widgets = dashboard.widgets.map((w) => (w.id === widget.id ? { ...w, ...span } : w));
      void persist({ ...dashboard, widgets });
    };

    handle.addEventListener("pointermove", move);
    handle.addEventListener("pointerup", end);
    handle.addEventListener("pointercancel", end);
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
      await refreshThemes();
    })();

    // 10s REST poll, the widget default from the spec. Also picks up
    // newly dropped plugin folders.
    const timer = setInterval(() => {
      // A gesture in flight owns the list: replacing it now would move the card
      // out from under the pointer.
      if (saveState === "saving" || arranging) return;
      fetchDashboard()
        .then((fresh) => (dashboard = fresh))
        .catch(() => {});
      void refreshPlugins();
      void refreshThemes();
    }, DASHBOARD_POLL_SECONDS * 1000);

    // A gesture that ends any way at all — a drop, a pointer cancel, a lost
    // capture — releases the refresh, so a cancelled drag cannot freeze it.
    const releaseGesture = () => {
      if (!resizing) arranging = false;
    };
    const measure = () => {
      viewportWidth = window.innerWidth;
    };
    window.addEventListener("pointerup", releaseGesture);
    window.addEventListener("pointercancel", releaseGesture);
    window.addEventListener("resize", measure);

    return () => {
      clearInterval(timer);
      window.removeEventListener("pointerup", releaseGesture);
      window.removeEventListener("pointercancel", releaseGesture);
      window.removeEventListener("resize", measure);
    };
  });
</script>

<div class="min-h-dvh bg-canvas font-sans text-body" data-part="app">
  <div class="mx-auto flex max-w-[1200px] flex-col px-4 py-6 sm:px-6">
    {#if dashboard}
      <div class="flex items-center justify-between gap-4" data-part="header">
        <StatusDot
          health={health}
          save={saveState}
          saveError={saveError}
          pollSeconds={DASHBOARD_POLL_SECONDS}
        />
        <div class="flex items-center gap-2">
          {#if editing}
            <select
              class="h-7 rounded-pill bg-surface-soft px-3 text-meta text-ink"
              value={activeTheme}
              onchange={(event) => chooseTheme(event.currentTarget.value)}
              aria-label="Theme"
            >
              {#each themes as theme (theme.slug)}
                <option value={theme.slug}>
                  {theme.name}{theme.present ? "" : " (file missing)"}
                </option>
              {/each}
            </select>
          {/if}
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

    <header class="flex items-center justify-center gap-3 py-10" data-part="identity">
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
                  {option.label}{option.present ? " (on the panel)" : ""}{option.problem
                    ? " (unavailable)"
                    : ""}
                </option>
              {/each}
            </select>
          </div>
        {/if}

        {#if editing}
          <section
            class="rounded-md border border-border bg-surface p-5"
            aria-labelledby="themes-heading"
          >
            <h2 id="themes-heading" class="text-subheading text-ink">Themes</h2>
            <p class="mt-1 text-meta text-mute">
              A Theme is a stylesheet. The default ships with the panel; the rest are CSS
              files in <span class="font-mono">themes/</span>.
            </p>

            <div class="mt-4 flex flex-wrap items-center gap-2">
              <input
                bind:this={fileInput}
                class="hidden"
                type="file"
                accept=".css,text/css"
                onchange={(event) => {
                  const file = event.currentTarget.files?.[0];
                  if (file) void importThemeFile(file);
                  event.currentTarget.value = "";
                }}
              />
              <button type="button" class="btn btn-secondary" onclick={() => fileInput?.click()}>
                Import a Theme…
              </button>
              {#if activeThemeIsImported}
                <a
                  class="btn btn-secondary"
                  href={themeHref(activeTheme)}
                  download={`${activeTheme}.css`}>Download {activeThemeName}</a
                >
              {:else}
                <button
                  type="button"
                  class="btn btn-secondary"
                  disabled
                  title="The default Theme is not a file, so there is nothing to download"
                  >Download</button
                >
              {/if}
            </div>

            {#if themeNotice}
              <p class="mt-3 text-meta text-mute">{themeNotice}</p>
            {/if}
            {#if themeError}
              <p class="mt-3 text-meta text-danger" data-part="error">{themeError}</p>
            {/if}

            {#if themes.length > 1}
              <ul class="mt-4 space-y-1">
                {#each themes as theme (theme.slug)}
                  {#if theme.slug !== "default"}
                    <li class="flex items-center justify-between gap-3 text-base text-body">
                      <span>
                        {theme.name}{theme.present ? "" : " (its file is gone)"}
                      </span>
                      <button
                        type="button"
                        class="btn btn-ghost px-2 text-meta"
                        aria-label="Delete {theme.name}"
                        onclick={() => removeTheme(theme)}>Delete</button
                      >
                    </li>
                  {/if}
                {/each}
              </ul>
            {/if}
          </section>
        {/if}

        <!-- Plugins: the same shape as Themes below, with a ZIP in place of a stylesheet. -->
        {#if editing}
          <section
            class="rounded-md border border-border bg-surface p-5"
            aria-labelledby="plugins-heading"
          >
            <h2 id="plugins-heading" class="text-subheading text-ink">Plugins</h2>
            <p class="mt-1 text-meta text-mute">
              A Plugin is a folder of code: a <span class="font-mono">manifest.json</span>,
              widget modules, and sometimes a program core starts with the panel's privileges.
              Import one as a ZIP of its folder, or download a folder to take it elsewhere.
              Importing runs code — the panel has no login, so keep it to a network you trust.
            </p>

            <div class="mt-4 flex flex-wrap items-center gap-2">
              <input
                bind:this={pluginFileInput}
                class="hidden"
                type="file"
                accept=".zip,application/zip"
                onchange={(event) => {
                  const file = event.currentTarget.files?.[0];
                  if (file) void importPluginFile(file);
                  event.currentTarget.value = "";
                }}
              />
              <button
                type="button"
                class="btn btn-secondary"
                onclick={() => pluginFileInput?.click()}
              >
                Import a Plugin…
              </button>
            </div>

            {#if pluginNotice}
              <p class="mt-3 text-meta text-mute">{pluginNotice}</p>
            {/if}
            {#if pluginProblem}
              <p class="mt-3 flex items-start gap-2 text-meta text-body" data-part="error">
                <Icon name="warning" class="mt-0.5 h-4 w-4 shrink-0 text-danger" />
                <span>It runs nowhere on this machine: {pluginProblem}.</span>
              </p>
            {/if}
            {#if pluginError}
              <p class="mt-3 text-meta text-danger" data-part="error">{pluginError}</p>
            {/if}

            {#if plugins.length > 0}
              <ul class="mt-4 space-y-1">
                {#each plugins as plugin (plugin.name)}
                  <li class="flex items-start justify-between gap-3 text-base text-body">
                    <span class="flex items-start gap-2">
                      {#if plugin.problem}
                        <Icon name="warning" class="mt-0.5 h-4 w-4 shrink-0 text-danger" />
                      {/if}
                      <span>
                        {plugin.name}
                        <span class="text-meta text-mute">{plugin.version}</span>
                        {#if plugin.problem}
                          <span class="block text-meta text-danger">Cannot run here: {plugin.problem}</span>
                        {/if}
                      </span>
                    </span>
                    <a
                      class="btn btn-ghost px-2 text-meta"
                      href={pluginArchiveHref(plugin.name)}
                      download={`${plugin.name}.zip`}
                      aria-label="Download {plugin.name}">Download</a
                    >
                  </li>
                {/each}
              </ul>
            {/if}
          </section>
        {/if}

        {#if dashboard.widgets.length === 0}
          <div class="py-14 text-center" data-part="empty">
            <p class="text-display text-ink">No widgets yet</p>
            <p class="mt-2 text-base text-body">
              {editing ? "Pick one above to add it." : "Enter edit mode to add one."}
            </p>
          </div>
        {:else}
          <ul
            class="dashboard-grid"
            bind:this={grid}
            use:dragHandleZone={{
              items: dashboard.widgets,
              flipDurationMs,
              dragDisabled: !canArrange,
              delayTouchStart: 150,
              dropTargetStyle: { outline: "none" }
            }}
            onconsider={handleConsider}
            onfinalize={handleFinalize}
          >
            {#each dashboard.widgets as w, i (w.id)}
              <li
                class={`${spanClasses(spanOf(w))}${isPlaceholder(w) ? " card-placeholder" : ""}`}
                animate:flip={{ duration: flipDurationMs }}
              >
                {#if !isPlaceholder(w)}
                <section
                  class="relative flex flex-col rounded-md border border-border bg-surface p-5"
                  aria-labelledby={`widget-${w.id}`}
                  data-part="card"
                >
                  <div class="flex items-start justify-between gap-3">
                    {#if editing}
                      <button
                        type="button"
                        class="btn btn-ghost card-handle px-2"
                        use:dragHandle
                        aria-label={`Move ${titleFor(w)}`}
                      >
                        <Icon name="dots-six-vertical" class="h-4 w-4" />
                      </button>
                    {/if}
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
                          class="btn btn-ghost px-2"
                          aria-label={`Remove ${titleFor(w)}`}
                          onclick={() => removeWidget(w)}
                        >
                          <Icon name="x" class="h-4 w-4" />
                        </button>
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
                      <div class="flex items-center gap-1 lg:hidden">
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
                      
                    </div>
                  {:else}
                    <p class="mt-4 text-meta text-mute">poll {w.pollSeconds}s</p>
                  {/if}

                  <!-- A mouse resizes from the edges themselves: the right edge
                       is the width, the bottom edge the height, and the corner
                       both. A coarse pointer gets the drawn handle above, since
                       an edge it cannot see is no affordance at all. -->
                  {#if editing}
                    <span
                      class="card-edge card-edge-right"
                      aria-hidden="true"
                      onpointerdown={(event) => startResize(event, w, "width")}
                    ></span>
                    <span
                      class="card-edge card-edge-bottom"
                      aria-hidden="true"
                      onpointerdown={(event) => startResize(event, w, "height")}
                    ></span>
                    <span
                      class="card-edge card-edge-corner"
                      aria-hidden="true"
                      onpointerdown={(event) => startResize(event, w, "both")}
                    ></span>
                  {/if}
                </section>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}

        <!-- Where the next card lands: the same frame as a card, and the same
             action as the select above it. -->
        {#if editing && plugins.length > 0}
          <button
            type="button"
            class="btn card-add"
            disabled={!addSelection}
            onclick={() => addWidget()}
            >+ {addLabel()}</button
          >
        {/if}

        {#if saveState === "error"}
          <section
            class="rounded-md border border-danger bg-surface p-5"
            aria-labelledby="save-error"
            data-part="error"
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
            data-part="error"
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
