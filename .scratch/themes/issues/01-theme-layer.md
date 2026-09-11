# 01: The Theme layer

**What to build:** a Theme as an imported CSS file: the folder, the name in the document, the endpoints that list, import, delete and serve one, and the `<link>` in the served `index.html` — with modes, the dark Palette and `ctx.theme` retired everywhere.

**Blocked by:** nothing.

**Status:** done

- [x] `themes/` resolves beside the binary exactly like `data/` and `plugins/`, with the same flag story (`-themes-dir`), and a CSS file dropped in by hand is listed without a restart
- [x] The Dashboard document carries `theme` as a name; an existing document migrates to `"default"` without losing anything it still uses
- [x] A Theme's name comes from `/* @name: … */`; a file without one gets a generated `theme-N`; files are written as `themes/<slug>.css`
- [x] `GET /api/v1/themes` lists the default first plus every imported Theme, marking the active one and flagging one whose file is gone
- [x] `POST /api/v1/themes` writes the body as CSS, rejects a name that already exists, and answers with the imported Theme
- [x] `GET /api/v1/themes/{name}.css` serves the file with `text/css`; an unknown name is 404; deleting the default is rejected
- [x] `DELETE /api/v1/themes/{name}` removes an imported Theme and the panel falls back to the default if it was active
- [x] The served `index.html` carries the active Theme's `<link>` before the app mounts, so there is no flash of the default
- [x] `mode`, `auto`, the dark Palette, the `prefers-color-scheme` defaults, the `[data-mode]` shadow and `ctx.theme` are gone from Go, `app.css`, `types.ts` and the widget context, and the Widget contract test still passes without a Theme field
- [x] `DESIGN.md`, `docs/PLUGINS.md` and the `README` describe the Theme that actually ships, and [ADR-0006](../../docs/adr/0006-themes-are-stylesheets.md) records that an imported Theme may load remote resources and that the contract is variables plus five attributes

## Comments

- 2026-09-12: Built as `internal/themes`, a module whose interface is `List`, `Find`, `Read`, `Add` and `Delete` over a folder, plus four endpoints and one line in the shell handler. The module was written test-first; the model change could not be, because retiring the Palette makes the tree uncompilable in one step, so its tests moved and changed with it.
- 2026-09-12: Two consequences worth knowing. A POST body has no file name, so "it is a CSS file" reduces to "there is something to import", with a 1 MB cap for a panel that should not hold an unbounded request; the name comes from the header comment and a file without one is written as `theme-N`. And a stylesheet *dropped into the folder* with no `@name` takes its file name as its display name, because a file we did not import has no stable number to invent — the generated name belongs to the import path, where it can be fixed before the file exists.
- 2026-09-12: The header has no Theme control yet: `ModeControl` is gone and its replacement is ticket 02, so between the two tickets the only way to change Theme is the API. That gap is deliberate and short.
- 2026-09-12: Verified on real data. The owner's document still held the old `{"mode","light","dark"}` object; it loaded as `default` without losing the widgets, which is the migration this ticket promised. Then a Theme imported from a file (`@name: Midnight`) reappeared in the list, became active, arrived in the served shell as a `<link>`, served as `text/css`, and repainted the panel and its Plugin widgets in the browser. The demo Theme was deleted and the Dashboard put back on the default afterwards.
- 2026-09-12: The README's dev command had been missing its folder flags since ticket 06 while the paragraph below insisted they mattered; fixed in passing.
