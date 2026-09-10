# 04: Chrome redesign

**What to build:** the Dashboard redrawn against `DESIGN.md` — centered identity, status dot with its detail, three-state mode control, view and edit modes, and the card, empty, loading and error states.

**Blocked by:** 02 tailwind-token-layer, 03 theme-v2-semantic-tokens.

**Status:** done

- [x] Header is two rows: status dot at the start and `Edit` plus mode control at the end, with the centered mark (32px) and wordmark below
- [x] Status dot maps `success` / `danger` / `mute` and reveals API, save and poll detail on hover and focus, with an accessible name
- [x] Mode control is a segmented light/dark/auto; `auto` follows `prefers-color-scheme` live and is the default for a fresh Dashboard
- [x] View mode shows a title, its body and one muted meta line; edit mode adds remove, enable and move controls per card plus the add-widget select, persisting each change as it happens
- [x] Cards, buttons, select, focus ring, empty, loading, error and widget-failure states follow `DESIGN.md`, with no `<style>` block or literal color left in the Svelte components
- [x] `README.md`'s Theme bullet describes the shipped look and points at `DESIGN.md`, and no comment in `apps/web` still cites `DESIGN-posthog.md`
- [x] Manual pass: light, dark and auto at three widths, keyboard-only navigation, and reduced motion

## Comments

- 2026-09-10: Components split into `App.svelte`, `WidgetCard.svelte` and `src/lib/{StarMark,StatusDot,ModeControl}.svelte`; all styling is Tailwind utilities plus a small `.btn` / `.control` family in `app.css`, so no Svelte `<style>` block survives and no component names a literal colour.
- 2026-09-10: Fixing the ticket-02 token layer was a prerequisite, not a detour: with `@theme inline` the radii and type steps were never emitted, so the variables `DESIGN.md` §5 promises Plugin authors did not exist. Constant tokens moved to `@theme static` (verified in the built CSS) and colours stayed inline.
- 2026-09-10: The manual pass caught a real design bug: dark `surface-soft` (`#1E2126`) was *lighter* than dark `surface` (`#17191C`), inverting the ramp against light mode and making the active segment of the mode control read as a hole rather than a raised pill. The dark value is now `#121417`, so `surface-soft` sits between `canvas` and `surface` in both modes. Corrected in `DESIGN.md`, the Go defaults, `app.css` and `dashboard.json`.
- 2026-09-10: Verified in the browser at desktop and 420px in light, dark and auto: the grid collapses to one column, the centered identity holds, the status popover opens on focus, the keyboard focus ring is visible, and the add flow persists a Widget end to end.
