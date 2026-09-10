<script lang="ts">
  import type { Theme } from "../types";

  interface Props {
    mode: Theme["mode"];
    onchange: (mode: Theme["mode"]) => void;
  }

  let { mode, onchange }: Props = $props();

  const options: Array<{ value: Theme["mode"]; label: string }> = [
    { value: "light", label: "Light" },
    { value: "dark", label: "Dark" },
    { value: "auto", label: "Auto" }
  ];
</script>

<div class="flex items-center gap-1 rounded-pill bg-surface-soft p-1" role="group" aria-label="Theme">
  {#each options as option (option.value)}
    <button
      type="button"
      class="h-7 rounded-pill px-3 text-meta transition {mode === option.value
        ? 'bg-surface text-ink'
        : 'text-mute hover:text-ink'}"
      aria-pressed={mode === option.value}
      onclick={() => onchange(option.value)}>{option.label}</button
    >
  {/each}
</div>
