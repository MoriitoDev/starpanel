<script lang="ts">
  interface Props {
    class?: string;
  }

  let { class: className = "" }: Props = $props();
</script>

<!-- The only logo asset: a four-point star in currentColor (DESIGN.md §4). -->
<svg class={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
  <path
    d="M12 1.5c.6 5.2 5.3 9.9 10.5 10.5-5.2.6-9.9 5.3-10.5 10.5C11.4 17.3 6.7 12.6 1.5 12 6.7 11.4 11.4 6.7 12 1.5Z"
  />
</svg>
