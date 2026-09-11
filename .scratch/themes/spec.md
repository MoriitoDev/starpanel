Status: ready-for-agent

## Problem Statement

The Dashboard ships exactly one look. A Theme is a fixed pair of Palettes chosen by a mode, so the owner can neither keep a look they like, try someone else's, nor change anything the fourteen tokens do not cover — borders, hovers, spacing, the shape of a card. Customising the panel means editing the app.

## Solution

A Theme becomes a CSS file the owner imports into `themes/`, layered on top of the baseline that ships in `app.css`. The Dashboard document stores only the name of the active Theme. Modes disappear with it: no light/dark selector, no `auto`, no second Palette. What a Theme author can rely on is written down as a contract: the token variables, the component classes, and five `data-part` attributes.

## User Stories

1. As a homelab owner, I want to import a Theme someone shared, so that the panel looks how I want without touching the app.
2. As a homelab owner, I want to switch between the default and my imported Themes, so that changing the look is one click.
3. As a homelab owner, I want to delete a Theme I no longer want, so that the list stays mine.
4. As a homelab owner, I want to download the active Theme, so that I can back it up or pass it on.
5. As a Theme author, I want plain CSS and a documented contract, so that I can restyle anything and know what keeps working.
6. As a Theme author, I want my Theme to be a single file, so that sharing it is copying it.
7. As a Plugin author, I want Widgets to keep working under any Theme, so that I can depend on the variables and nothing else.

## Implementation Decisions

- A Theme is a CSS file in `themes/`, resolved beside the binary exactly like `data/` and `plugins/` (same rule, same flag story).
- The Dashboard document stores the active Theme as a name (`"theme": "default"`). The default is not a file: it is the baseline in `app.css`, always first in the list.
- Retired with the modes: the dark Palette, the `prefers-color-scheme` defaults, the `[data-mode]` popover shadow, `Theme.Mode` in Go, and `ctx.theme` in the Widget context.
- A Theme's name comes from a `/* @name: … */` header comment. A file without one gets a generated `theme-N`, and the file is written as `themes/<slug>.css`.
- Endpoints: `GET /api/v1/themes` lists, `GET /api/v1/themes/{name}.css` serves (and is what the download button points at), `POST /api/v1/themes` imports raw CSS in the body, `DELETE /api/v1/themes/{name}` removes one.
- Importing a name that already exists is rejected; the default cannot be deleted.
- The server writes the active Theme's `<link>` into `index.html` when it serves it, so the panel never paints the default first and flips.
- A missing or deleted active Theme falls back to the default, and the list marks it as absent.
- Import validation is "it is a CSS file": remote `@import` and `url()` are allowed, because the panel's default Theme ships none and the author's freedom beats our guarantee.

## Testing Decisions

- Seam 1 (HTTP): list, import, delete and serve round-trip; a duplicate import is rejected; an unknown name 404s; a deleted active Theme still serves a working panel on the default; `index.html` arrives with the active Theme's `<link>`.
- Seam 2 (theme files): the header is read when present and a name generated when it is not; the folder is rescanned per request, so a hand-dropped file appears without a restart.
- Frontend: `pnpm check` stays clean; the picker and the Themes section are verified by hand in the browser, in the default Theme and in an imported one.

## Out of Scope

- An editor or a colour picker. Authoring a Theme happens in a text editor; the panel imports, lists, deletes and downloads.
- Exporting every Theme as a pack; only the active one downloads.
- A per-Theme light/dark switch or following the system: a Theme that wants that writes `@media (prefers-color-scheme)` itself.
- Assets, subfolders or multiple files per Theme.
- Sanitising or sandboxing Theme CSS beyond requiring a `.css` file.

## Further Notes

- Decisions come from a grilling session on 2026-09-11; `CONTEXT.md`'s Theme and Palette entries are already updated to match.
- [ADR-0006](../../docs/adr/0006-themes-are-stylesheets.md) records the two decisions a future explorer would otherwise re-propose.
