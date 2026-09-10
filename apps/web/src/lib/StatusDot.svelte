<script lang="ts">
  interface Props {
    health: "checking" | "alive" | "degraded" | "offline";
    save: "saved" | "saving" | "error";
    saveError?: string | null;
    pollSeconds: number;
  }

  let { health, save, saveError = null, pollSeconds }: Props = $props();

  let state = $derived(
    health === "alive" ? "online" : health === "checking" ? "checking" : health
  );
  let tone = $derived(health === "alive" ? "success" : health === "checking" ? "mute" : "danger");
  let dotClass = $derived(
    tone === "success" ? "bg-success" : tone === "danger" ? "bg-danger" : "bg-mute"
  );
  let saveLabel = $derived(
    save === "saving" ? "saving" : save === "error" ? (saveError ?? "failed") : "saved"
  );
  // The accessible name carries the whole detail, so the popover is a pointer
  // affordance rather than the only way to read it (DESIGN.md §7).
  let summary = $derived(`API ${state} · ${saveLabel} · polling every ${pollSeconds}s`);
  let rows = $derived([
    { label: "API", value: state },
    { label: "Save", value: saveLabel },
    { label: "Poll", value: `every ${pollSeconds}s` }
  ]);
</script>

<div class="group relative">
  <button
    type="button"
    class="flex h-8 w-8 items-center justify-center rounded-pill"
    aria-label={summary}
  >
    <span class="h-2 w-2 rounded-pill {dotClass}"></span>
  </button>
  <div
    class="pointer-events-none absolute left-0 top-full z-10 hidden w-64 rounded-lg border border-border bg-surface p-3 shadow-popover group-hover:block group-focus-within:block"
    aria-hidden="true"
  >
    <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-meta">
      {#each rows as row (row.label)}
        <dt class="text-mute">{row.label}</dt>
        <dd class="truncate text-ink">{row.value}</dd>
      {/each}
    </dl>
  </div>
</div>
