# Star Panel — Design System

Normative source of truth for the web UI's visual language: the Dashboard chrome in `apps/web` and, through [docs/PLUGINS.md](./docs/PLUGINS.md), every Plugin widget. If code and document disagree, one of them changes in the same commit — nothing here is silently ignored.

Every value below is declared once in `apps/web/src/app.css` (see [ADR-0004](./docs/adr/0004-tailwind-with-one-token-source.md)) and referenced by name. A literal color, radius or font size inside a component is a bug.

## 1. Principles

**Content first.** The panel is an instrument the owner glances at. Chrome recedes: no illustration, no ornament, no decorative gradient, no icon that does not replace a word.

**Hairlines, not boxes.** Structure comes from 1px borders at low contrast plus whitespace. Depth is reserved for popovers; cards never float.

**One accent.** The accent marks the primary action on screen and links. `danger` and `success` report state and are never used for emphasis.

**Air over density.** Hierarchy comes from the type scale and text color, not from a rule drawn between every element.

**Quiet motion.** 140ms, opacity and transform only, no bounce. A panel that re-polls every 10s must not move on its own.

**Neutral by default, tinted by the owner.** The grayscale is fixed. Surfaces and accent are the owner's data, stored in the Theme.

**What we are not copying.** The restraint is inspired by DeepSeek's documentation and Apple's product pages (roughly 70/30), but not their layouts, hues, typefaces, compositions or assets. We take the discipline — few colors, hairlines, large calm type, discreet movement — and derive our own values. Nothing in the product should be recognizable as a specific page.

## 2. Tokens

### Color

| Token | Light | Dark | Role |
| --- | --- | --- | --- |
| `canvas` | `#F7F8F9` | `#0E1012` | The page behind everything |
| `surface` | `#FFFFFF` | `#17191C` | Cards, popovers, inputs |
| `surface-soft` | `#F2F3F5` | `#121417` | Inset areas, secondary controls, hover on a surface |
| `border` | `#E5E7EB` | `#2A2E34` | The default 1px edge |
| `border-soft` | `#EFF1F3` | `#22262B` | Edges that should barely register (rows inside a card) |
| `ink` | `#0D0F12` | `#F3F4F6` | Titles and figures |
| `body` | `#40454D` | `#C7CBD1` | Running text |
| `mute` | `#8B919B` | `#878D96` | Meta, labels, disabled and unknown states |
| `accent` | `#4F6BFF` | `#7C8DFF` | Primary action, links, active segment |
| `accent-press` | `#3D57EE` | `#6A7CFF` | The pressed state of anything carrying the accent |
| `on-accent` | `#FFFFFF` | `#0E1012` | Text and glyphs on `accent` |
| `danger` | `#E5484D` | `#FF6369` | Failed, offline, destructive |
| `success` | `#2E9E68` | `#3DD68C` | Healthy, running |
| `focus-ring` | `rgba(79,107,255,.45)` | `rgba(124,141,255,.5)` | The only focus indicator |

These fourteen are the only colors in the product, chrome and Plugin widgets alike. One value per token per mode forms a **Palette**; the mode plus the two Palettes is the **Theme** ([CONTEXT.md](./CONTEXT.md)). There is deliberately no warning hue: a state is fine (`success`), not fine (`danger`), or not known yet (`mute`).

### Shape and depth

| Token | Value | Use |
| --- | --- | --- |
| `radius-sm` | `8px` | Buttons, inputs, selects, inline chips |
| `radius-md` | `12px` | Cards |
| `radius-lg` | `16px` | Popovers and large surfaces |
| `radius-pill` | `999px` | Icon-only controls, segmented controls |

Borders are always 1px; apparent weight comes from color, never from width. There is exactly one shadow, on popovers: `0 8px 24px rgb(0 0 0 / .10)` (dark: `0 8px 24px rgb(0 0 0 / .45)`). Cards, headers and buttons cast nothing.

### Type

Family: **Geist Sans** (variable, latin subset, self-hosted), falling back to `system-ui, -apple-system, "Segoe UI", sans-serif`. Numeric data uses tabular figures.

| Step | Size / line-height | Weight | Use |
| --- | --- | --- | --- |
| `display` | 34 / 40 | 600 | Panel identity, empty-state headline |
| `heading` | 24 / 32 | 600 | Section titles |
| `subheading` | 18 / 26 | 600 | Card titles |
| `base` | 15 / 24 | 400 | Running text and values |
| `meta` | 13 / 20 | 500 | Labels, plugin/status/poll metadata, disabled text |

`display`, `heading` and `subheading` carry -0.02em tracking; the rest carry none. At most two weights appear in one view. Monospace is a family rather than a step: code-like content is `font-mono` at `meta` size. The step is `base` and not `body` because `body` is already a colour.

### Motion

Duration 140ms, easing `cubic-bezier(.2, 0, 0, 1)`. Only `opacity`, `transform` and color-family properties animate. `prefers-reduced-motion: reduce` drops every duration to 0ms.

### Spacing

The default scale, used from this set only: 4, 8, 12, 16, 20, 24, 32, 48, 64. Card padding is 20px (16px under 640px); the gap between major sections is 32px.

## 3. Rules

- Reference a Token by name; never write a hex, an `rgb()`, a radius or a font size in px inside a component.
- No `dark:` variants. Modes are the Palette layer's job, so no component knows which mode is active.
- One accent per screen. No colored borders, no tinted background behind text, no gradients anywhere.
- A card is `surface` + 1px `border` + `radius-md`. No shadow, no hover lift; hover changes background, never position.
- Never nest more than two surfaces (canvas → surface → surface-soft).
- Focus is never removed. The indicator is a 2px `focus-ring` ring with a 2px offset, on `:focus-visible` only, so a mouse click does not ring.
- Disabled means `mute` text on `surface-soft` — never a dimmed accent, never a faded copy of an enabled control.
- Nothing is communicated by color alone: the status dot carries a label, an error carries words.
- Body text never goes below 15px, meta never below 13px.
- Every state is designed: empty, loading, error and edit are deliberate surfaces, not browser defaults.
- Nothing animates on its own.

## 4. Components

**App header.** Two rows. The first holds the status dot at the start and, at the end, the `Edit` control plus the mode control. The second holds the centered identity: the mark at 32px followed by the wordmark "Star Panel" in `display`, tracked -0.02em. Under 640px the rows stay two: the identity stays centered and the controls keep their corners.

**Identity mark.** A single-color four-point star (inline SVG, `currentColor`), 32px in the header, the same shape as the favicon. It is the only logo asset — no images, no mascot.

**Status dot.** An 8px circle: `success` when the API answers, `danger` when it does not, `mute` while checking. Hover or keyboard focus reveals a popover (`surface`, `radius-lg`, the one shadow) with the API state, the save state and the poll interval. The dot has an accessible name and the detail is reachable without a pointer.

**Mode control.** Three states — light, dark, auto — as a segmented control: `radius-pill` track on `surface-soft`, active segment on `surface` with `ink` text. `auto` follows `prefers-color-scheme` live and is what a fresh Dashboard gets.

**Card.** `surface`, 1px `border`, `radius-md`, 20px padding, 16px between the title block and the body. Title in `subheading`/`ink`, metadata in `meta`/`mute`. In view mode a card shows a title, its body and at most one muted metadata line. Nothing else.

**Edit mode.** Toggled from the header, where `Edit` becomes a filled `Done`. Edit mode adds to each card a control cluster at the top end (remove), an `enabled` switch, and move controls at the bottom end; it also reveals the add-widget select, which is hidden while viewing. Each change persists as it is made, as today.

**Buttons.** Primary: `accent` background, `on-accent` text, `radius-sm`, 36px tall, 14px horizontal padding, weight 500, `accent-press` while pressed. Secondary: `surface` + 1px `border` + `ink`. Ghost: transparent + `mute`, no border. Disabled per the Rules.

**Select.** The metrics of a secondary button: `surface`, 1px `border`, `radius-sm`.

**Empty state.** Centered: a `display` headline in `ink`, one `body` sentence in `body`, and the add-widget control when plugins exist. No illustration.

**Loading.** The shell renders with the palette applied and the identity in place; content areas show `mute` placeholder text, never a spinner.

**Error.** Rejected plugin folders and failed saves render on `surface` with a 1px border — `border` for an informational note, `danger` when something actually failed — and the failure is named in words. `danger` colors the mark or the label, not a whole tinted panel.

**Widget failure.** A widget that cannot load or poll shows one `meta` line in `danger` inside its card: `Widget failed: <reason>`. The card stays and the panel keeps working (ADR-0003).

## 5. Plugin widgets

Plugin widgets are vanilla ESM rendered at runtime, so Tailwind's build-time scanner cannot see them and they must not use utility classes. Their contract is the Token layer, documented for authors in [docs/PLUGINS.md](./docs/PLUGINS.md): the fourteen colors, the four radii and the type steps, consumed as CSS variables (`var(--accent)`). A widget inherits the active Palette from the Dashboard and never hardcodes a color, a font or a shadow.

## 6. Layout and responsiveness

Content is at most 1200px wide, centered, with 24px gutters (16px under 640px).

| Breakpoint | Grid | `small` | `medium` | `large` |
| --- | --- | --- | --- | --- |
| ≥1024px | 12 columns, 24px gaps | 4 | 6 | 12 |
| 640–1023px | 6 columns, 16px gaps | 3 | 6 | 6 |
| <640px | 1 column | 1 | 1 | 1 |

`small` / `medium` / `large` is the Widget's declared size and the only layout control a user has; there is no drag grid.

## 7. Accessibility

- Body text meets 4.5:1 and large or UI text meets 3:1 in both Palettes; `mute` is for non-essential text only.
- Focus is always visible (`:focus-visible`, 2px ring, 2px offset) and never clipped by `overflow: hidden`.
- Controls are at least 44px on a coarse pointer and at least 32px on a fine one.
- Status is never color-only; the dot and every state label carry text or an accessible name.
- Motion respects `prefers-reduced-motion: reduce`.
- Structure is semantic: one `h1` for the panel identity, each card a labelled `section`, controls as `button` or `label` + `input`, never clickable `div`s.
- The document is `lang="en"` and the UI copy is English.

## 8. Implementation notes

- Tailwind v4 through `@tailwindcss/vite`, imported once from `apps/web/src/main.ts`. One stylesheet, `apps/web/src/app.css`, holds the `@font-face`, the `:root` light defaults and the `@theme inline` mapping. No Svelte `<style>` blocks and no `style="..."` attributes, except the Palette injection below.
- Runtime theming: `:root` declares the light Palette, and `prefers-color-scheme: dark` the dark one, so the shell renders correctly before the Dashboard loads; `@theme inline` maps each Token to `var(--token)` so utilities compile to the variable and stay dynamic; `App.svelte` writes the active Palette inline on the app shell, as it does today with a shorter list. Those defaults restate the Palette values, so a change to the Theme defaults belongs in three places in one commit: `DESIGN.md`, `app.css` and the Go defaults.
- Naming: CSS custom properties are kebab-case (`--surface-soft`); the JSON, TypeScript and Go fields are their camelCase twins (`surfaceSoft`).
- Theme shape: `mode` ∈ `light | dark | auto`, plus a light and a dark Palette of these fourteen colors. Radii, type, spacing and motion are not part of the Theme.
- Font: Geist Sans variable under `apps/web/public/fonts/`, with `OFL.txt` beside it (Vite copies `public/` into `dist/`), `font-display: swap`, preloaded in `index.html`. The shipped face is the full charset at ~68 KB rather than a subset: subsetting would modify the font, which the OFL's reserved-name clause makes worth avoiding. Geist Mono is not shipped; numbers use tabular figures.
- Plugin widgets keep the CSS-variable contract of [ADR-0003](./docs/adr/0003-plugin-subprocess-vanilla-esm.md).
- The default Tailwind colour palette is switched off (`--color-*: initial`), so the fourteen tokens are the only colours a utility can name.
- Sources are declared explicitly (`@source "../src"`, `@source "../index.html"`) because the repo ships no `.gitignore`: automatic detection would also scan `dist/` and feed the built CSS back into the next build.
- `--popover-shadow` is a system value rather than a Theme token: `app.css` ships a light and a dark value and the app shell picks one by marking the resolved mode with `data-mode`.
