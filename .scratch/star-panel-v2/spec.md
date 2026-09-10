Status: ready-for-agent

## Problem Statement

The Dashboard works, but it reads as a prototype: all styling lives in one `<style>` block inside `App.svelte`, the Theme speaks a 29-color vocabulary inherited from a design file that is not in the repo, every card permanently shows its editing controls, and the copy mixes English and Spanish. No design decision is written down, so each new component re-invents one by imitating existing markup.

## Solution

Write the design system down and make it executable. `DESIGN.md` becomes normative; Tailwind v4 plus a single token stylesheet turns it into the only source of color, radius and type in the app; the Theme shrinks to fourteen semantic colors plus a `light | dark | auto` mode; the chrome is redrawn around a centered identity, a status dot and a view/edit split; and the three bundled widgets are restyled against the same tokens with their contract documented for Plugin authors.

## User Stories

1. As a homelab owner, I want a panel that looks deliberate at a glance, so that the thing I look at every day is calm instead of noisy.
2. As a homelab owner, I want to change one value in one file to restyle the whole panel, so that adjusting the look is not a hunt through components.
3. As a homelab owner, I want the panel to follow my system's light/dark preference, so that it matches the rest of my desktop without me touching it.
4. As a homelab owner, I want the panel's color identity to stay mine, so that it fits my setup.
5. As a homelab owner, I want a view mode where controls do not compete with the data, and an edit mode when I am changing things.
6. As a Plugin author, I want the styling contract written down, so that my widget looks native without guessing.
7. As a developer, I want a short token vocabulary, so that a palette change is a value change and not a refactor.
8. As a developer, I want the design rules in the repo, so that a new component has one place to check.

## Implementation Decisions

- The design system lives in a root `DESIGN.md` (principles, tokens, rules, components, plugin pointer, layout, accessibility, implementation) and supersedes the referenced-but-absent `DESIGN-posthog.md`.
- Tailwind v4 with `@tailwindcss/vite`, one tokens stylesheet, `@theme inline` so runtime Palettes keep driving utilities (ADR-0004).
- Theme v2: `mode` ∈ `light | dark | auto` plus fourteen semantic colors per Palette. Radii, type, spacing and motion are build-time constants and are not configurable.
- Stored v1 palettes are not migrated: a Palette missing any token is replaced wholesale by the `DESIGN.md` defaults, which resets every existing document to the new Palette. Merging token by token would keep v1 colours alive under the six names both token sets share.
- The legacy palette machinery (`expandOld`, `hasOld`, the three-color aliases) is deleted rather than extended.
- Bundled widgets keep their vanilla-ESM + CSS-variable contract (ADR-0003); Tailwind is chrome-only, because the scanner runs at build time and cannot see a plugin module loaded at runtime.
- Geist Sans variable, self-hosted under `apps/web/public/fonts/`, latin subset, OFL. No font CDN: the panel stays fully offline.
- Identity is a four-point star as inline SVG, reused as the favicon; the only logo asset.
- UI copy becomes English throughout, including the strings inside the bundled widgets.
- Widget sizes stay `small` / `medium` / `large` on a 12/6/1 column grid; no drag grid.
- Edit mode is explicit (a header toggle) rather than hover-revealed controls, so it works on touch.

## Testing Decisions

- Seam 1 (HTTP v1): Dashboard get/save round-trips the v2 Theme; validation rejects an unknown mode and an incomplete Palette; `auto` is accepted; a v1 document on disk loads as the new defaults. Existing tests are updated, not replaced.
- Seam 2 (Plugin contract): the widget contract tests still pass, and every bundled widget still renders with a stub element and stub context.
- Frontend: `pnpm check` stays clean and `pnpm build` produces the bundle.
- Manual visual verification in light, dark and auto at three widths, plus a keyboard-only pass and a reduced-motion pass.
- Explicitly untested: aesthetic judgment. The panel has no visual regression tests and this work does not add them; look and feel is reviewed by eye against `DESIGN.md`.

## Out of Scope

- A UI for editing the Palette; the Theme stays editable through JSON and the API.
- `docs/PLUGINS.md` as a full authoring guide (Manifest, backend, subprocess); v2 documents the styling contract only.
- Drag-and-drop layout, per-device layouts, or a free 12-column span model.
- Internationalisation or any copy beyond English.
- The single-binary embed (v1 ticket 06); the font lands under `apps/web/public/` so that ticket inherits it.
- New Plugins or Widgets, history, graphs, or any database.
- Geist Mono, icon libraries, illustrations or mascot assets.

## Further Notes

- The decisions here come from a grilling session on 2026-09-10. `CONTEXT.md` gained `Palette` and `Token` and sharpened `Theme`.
- Respects ADR-0003 and ADR-0004; leaves ADR-0001 and ADR-0002 untouched.
- `DESIGN.md` is normative: when code and document disagree, one of them changes in the same commit.
- v1 tickets 01–05 are done and ticket 06 is not, so there is still no embedded frontend to keep in sync.
