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

| Token | Default | Role |
| --- | --- | --- |
| `canvas` | `#F7F8F9` | The page behind everything |
| `surface` | `#FFFFFF` | Cards, popovers, inputs |
| `surface-soft` | `#F2F3F5` | Inset areas, secondary controls, hover on a surface |
| `border` | `#E5E7EB` | The default 1px edge |
| `border-soft` | `#EFF1F3` | Edges that should barely register (rows inside a card) |
| `ink` | `#0D0F12` | Titles and figures |
| `body` | `#40454D` | Running text |
| `mute` | `#8B919B` | Meta, labels, disabled and unknown states |
| `accent` | `#4F6BFF` | Primary action, links, active segment |
| `accent-press` | `#3D57EE` | The pressed state of anything carrying the accent |
| `on-accent` | `#FFFFFF` | Text and glyphs on `accent` |
| `danger` | `#E5484D` | Failed, offline, destructive |
| `success` | `#2E9E68` | Healthy, running |
| `focus-ring` | `rgba(79,107,255,.45)` | The only focus indicator |

This is the **Palette** the default Theme ships: the fourteen values the chrome and every Plugin widget are built on, and the only colours a utility can name. A **Theme** is a stylesheet that repaints any of them — or anything else — and the owner can import one into `themes/` ([ADR-0006](./docs/adr/0006-themes-are-stylesheets.md)). There is deliberately no warning hue: a state is fine (`success`), not fine (`danger`), or not known yet (`mute`).

### Shape and depth

| Token | Value | Use |
| --- | --- | --- |
| `radius-sm` | `8px` | Buttons, inputs, selects, inline chips |
| `radius-md` | `12px` | Cards |
| `radius-lg` | `16px` | Popovers and large surfaces |
| `radius-pill` | `999px` | Icon-only controls, segmented controls |

Borders are always 1px; apparent weight comes from color, never from width. There is exactly one shadow — `0 8px 24px rgb(0 0 0 / .10)` — on popovers, and on a Widget while its owner is dragging it. At rest, cards, headers and buttons cast nothing. A Theme may change the shadow, and one that paints a dark canvas will want to.

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

### Grid

| Token | Default | Role |
| --- | --- | --- |
| `row-height` | `120px` | One grid row: a Widget's height is its rows times this, and that height is a minimum, never a ceiling. Rows are spaced by the gap of §6, which is what a corner drag snaps to. |

The column count and the gaps belong to the breakpoints, and this is the one length the layout is measured in: the CSS sets it, and the drag interaction reads it rather than repeating the number (§8).

## 3. Rules

- Reference a Token by name; never write a hex, an `rgb()`, a radius or a font size in px inside a component.
- No `dark:` variants. A Theme's own CSS decides how it reacts to anything, `prefers-color-scheme` included.
- One accent per screen. No colored borders, no tinted background behind text, no gradients anywhere.
- A card is `surface` + 1px `border` + `radius-md`. No shadow, no hover lift; hover changes background, never position. While its owner is dragging it, a card carries the one shadow of §2 and the place it will land is marked by a placeholder.
- Never nest more than two surfaces (canvas → surface → surface-soft).
- Focus is never removed. The indicator is a 2px `focus-ring` ring with a 2px offset, on `:focus-visible` only, so a mouse click does not ring.
- Disabled means `mute` text on `surface-soft` — never a dimmed accent, never a faded copy of an enabled control.
- Nothing is communicated by color alone: the status dot carries a label, an error carries words.
- Body text never goes below 15px, meta never below 13px.
- Every state is designed: empty, loading, error and edit are deliberate surfaces, not browser defaults.
- Nothing animates on its own.

## 4. Components

**App header.** Two rows. The first holds the status dot at the start and the `Edit` control at the end, with the Theme control beside it while editing. The second holds the centered identity: the mark at 32px followed by the wordmark "Star Panel" in `display`, tracked -0.02em. Under 640px the rows stay two: the identity stays centered and the controls keep their corners.

**Identity mark.** A single-color four-point star (inline SVG, `currentColor`), 32px in the header, the same shape as the favicon. It is the only logo asset — no images, no mascot.

**App shell.** The root element carries `data-part="app"` and paints `canvas`. It is the hook a Theme uses for a wallpaper, and the reason `surface` has to stay opaque if the wallpaper is to stay behind the cards.

**Status dot.** An 8px circle: `success` when the API answers, `danger` when it does not, `mute` while checking. Hover or keyboard focus reveals a popover (`surface`, `radius-lg`, the one shadow) with the API state, the save state and the poll interval. The dot has an accessible name and the detail is reachable without a pointer.

**Theme control.** Edit mode only: choosing a look is editing, and view mode shows the result rather than the controls that produced it. A compact select on a `surface-soft` `radius-pill`, showing the active Theme's name and opening the list — the default first, then every stylesheet in `themes/`, with a file-missing note for one whose file has gone. Picking one persists its name in the Dashboard and swaps the stylesheet link in place, so nothing reloads. Importing, deleting and downloading live in the Themes section below.

**Card.** `surface`, 1px `border`, `radius-md`, 20px padding, 16px between the title block and the body. Title in `subheading`/`ink`, metadata in `meta`/`mute`. In view mode a card shows a title, its body and at most one muted metadata line. Nothing else.

**Edit mode.** Toggled from the header, where `Edit` becomes a filled `Done`. Edit mode turns each card into something the owner can arrange: a drag handle at the top start, the `enabled` switch and a remove control — the Phosphor cross — at the top end, and, on a mouse, the edges themselves: the right edge changes the width, the bottom edge the height and the corner both, each grabbable across the whole gap that separates two cards, since that band is what an owner reads as the edge, with a drawn handle under the pointer where the pointer is coarse. Dragging a card reorders the grid, marking the landing place with a dashed `border` on `surface-soft`, and the card under the pointer carries the one shadow until it is dropped. Under 1024px the `↑`/`↓` move controls appear beside those handles, and under 640px they are the only way to move a card and no edge or handle resizes a card: a one-column grid has nothing to resize across. Edit mode also reveals the add control — a select naming the Widget, and a `+` control under the grid that adds the named Widget at the end, ready to be dragged into place — a Themes section that imports, deletes with a confirmation and downloads the active Theme, and a Plugins section that imports a ZIP of a Plugin's folder and downloads any Plugin already there. Each change persists as it is made, as today.

**Buttons.** Primary: `accent` background, `on-accent` text, `radius-sm`, 36px tall, 14px horizontal padding, weight 500, `accent-press` while pressed. Secondary: `surface` + 1px `border` + `ink`. Ghost: transparent + `mute`, no border. Disabled per the Rules.

**Select.** The metrics of a secondary button: `surface`, 1px `border`, `radius-sm`.

**Empty state.** Centered: a `display` headline in `ink`, one `body` sentence in `body`, and the add-widget control when plugins exist. No illustration.

**Loading.** The shell renders with the palette applied and the identity in place; content areas show `mute` placeholder text, never a spinner.

**Error.** Rejected plugin folders and failed saves render on `surface` with a 1px border — `border` for an informational note, `danger` when something actually failed — and the failure is named in words. `danger` colors the mark or the label, not a whole tinted panel.

**Widget failure.** A widget that cannot load or poll shows one `meta` line in `danger` inside its card: `Widget failed: <reason>`. The card stays and the panel keeps working (ADR-0003).

**Plugins section.** Edit mode lists every Plugin by name and version, each with a Download control, and a control that imports one from a ZIP of its folder. The import says what it is before the file picker is reached: what arrives is code the panel will run, and the panel has no login ([ADR-0007](./docs/adr/0007-plugin-import-is-an-upload-of-code.md)). A Plugin whose `requires` are not met carries the Phosphor warning icon in `danger` on its row, with the reason in `danger` meta text underneath, and an import of one says the same thing in its answer. The panel never installs anything: it says what is missing, and `docs/PLUGINS.md` says what to do about it.

**Icons.** They come from [Phosphor](https://phosphoricons.com/) and are inlined in `apps/web/src/lib/Icon.svelte` rather than fetched, so the panel works offline and ships no icon runtime. Copy the path from the site when you need another one; never load an icon from a CDN. The identity mark is the one exception — it is ours, and it lives in `StarMark.svelte`.

## 5. Plugin widgets

Plugin widgets are vanilla ESM rendered at runtime, so Tailwind's build-time scanner cannot see them and they must not use utility classes. Their contract is the Token layer, documented for authors in [docs/PLUGINS.md](./docs/PLUGINS.md): the fourteen colors, the four radii and the type steps, consumed as CSS variables (`var(--accent)`). A widget inherits whatever Theme is active and never hardcodes a color, a font or a shadow; the Tokens it reads are the default Theme's, and any Theme may repaint them.

## 6. Layout and responsiveness

Content is at most 1200px wide, centered, with 24px gutters (16px under 640px).

| Breakpoint | Grid | The width a Widget's Span draws at |
| --- | --- | --- |
| ≥1024px | 12 columns, 24px gaps | its own `w` |
| 640–1023px | 6 columns, 16px gaps | `w` halved and rounded, at least 1 column |
| <640px | 1 column | 1 column |

A Widget's **Span** is the only layout control its owner has: `w` columns (1–12) and `h` rows (1–12, each `--row-height`). The Dashboard stores its Widgets in order and the grid flows them, so a Span says how big a card is and never where it sits: the grid fills each row in order, and a row can end short of the full width. `h` is a minimum: a card whose content needs more room grows to fit it rather than clipping, the drag only ever enlarges it, and a card never borrows a neighbour's height — a row is as tall as its tallest card, and the shorter one keeps its own size with the difference left empty.

Under 1024px the stored layout reflows to the narrower grid as the table shows, and under 640px the stored `h` is ignored: every card is one column wide and as tall as its content. In edit mode the owner drags a card by its handle to reorder it and drags the edge of a card to change its Span ([ADR-0008](./docs/adr/0008-layout-is-an-ordered-grid.md)); the change is saved the moment the gesture ends.

## 7. Accessibility

- The default Theme meets 4.5:1 for body text and 3:1 for large or UI text, and `mute` is for non-essential text only. An imported Theme is its author's business: the panel checks nothing about it.
- Focus is always visible (`:focus-visible`, 2px ring, 2px offset) and never clipped by `overflow: hidden`.
- Controls are at least 44px on a coarse pointer and at least 32px on a fine one.
- Status is never color-only; the dot and every state label carry text or an accessible name.
- Motion respects `prefers-reduced-motion: reduce`.
- Structure is semantic: one `h1` for the panel identity, each card a labelled `section`, controls as `button` or `label` + `input`, never clickable `div`s.
- Arranging the Dashboard by drag takes a pointer; below 1024px every card keeps its `↑`/`↓` buttons, and every control edit mode adds is a `button` or a `label` + `input`.
- The document is `lang="en"` and the UI copy is English.

## 8. Implementation notes

- Tailwind v4 through `@tailwindcss/vite`, imported once from `apps/web/src/main.ts`. One stylesheet, `apps/web/src/app.css`, holds the `@font-face`, the baseline Palette in `:root` and the `@theme inline` mapping. No Svelte `<style>` blocks and no `style="..."` attributes.
- Runtime theming: `app.css` declares the baseline, and `@theme inline` maps each Token to `var(--token)` so utilities compile to the variable and a Theme can repaint it. The server writes the active Theme's `<link>` into `index.html` as it serves it, so the panel never paints the baseline first and flips. Changing the baseline means changing `DESIGN.md` and `app.css` together; an imported Theme changes nothing but itself.
- Naming: CSS custom properties are kebab-case (`--surface-soft`); the JSON, TypeScript and Go fields are their camelCase twins (`surfaceSoft`).
- Theme shape: a Dashboard stores the *name* of the Theme it renders with. `default` is the baseline in `app.css`, which ships inside the binary and cannot be deleted; every other name is a stylesheet in `themes/`, beside the binary like `data/` and `plugins/`. A Theme may set any variable and any rule it likes; the fourteen Tokens, the component classes in `app.css` and the `data-part` attributes are what the panel promises to keep.
- Font: Geist Sans variable under `apps/web/public/fonts/`, with `OFL.txt` beside it (Vite copies `public/` into `dist/`), `font-display: swap`, preloaded in `index.html`. The shipped face is the full charset at ~68 KB rather than a subset: subsetting would modify the font, which the OFL's reserved-name clause makes worth avoiding. Geist Mono is not shipped; numbers use tabular figures.
- Plugin widgets keep the CSS-variable contract of [ADR-0003](./docs/adr/0003-plugin-subprocess-vanilla-esm.md).
- The default Tailwind colour palette is switched off (`--color-*: initial`), so the fourteen tokens are the only colours a utility can name.
- Sources are declared explicitly (`@source "../src"`, `@source "../index.html"`) because the repo ships no `.gitignore`: automatic detection would also scan `dist/` and feed the built CSS back into the next build.
- `--popover-shadow` is a baseline value like the rest: `app.css` sets it, and a Theme that paints a dark canvas overrides it.
- The grid is described once, in `app.css`: its column template, the breakpoint gaps and `--row-height`. The arithmetic in `apps/web/src/lib/layout.ts` mirrors the two breakpoints of §6, reads the gap and the row height from the DOM, and is pinned by cases in `apps/web/tests/layout.test.mjs` — so a grid that changes without the arithmetic changing fails a test instead of confusing a drag. The drag states (the lifted card, the placeholder) are classes in `app.css` like every other state in the panel.
