# hello-widget

The reference frontend-only Plugin — copy this folder to add your own.

1. Copy the `hello-widget` folder and rename it (folder name must equal the
   Manifest `name`).
2. Edit `manifest.json`: name, version, and the widgets you provide.
3. Edit `widget.js`: render into the given element with the given context.

A Plugin folder is a Widget host contract, not a build step: the module is
framework-free ESM served as-is. The default export receives
`(el, ctx)` where `ctx` carries `config` (free-form JSON from the
Dashboard), `theme` variables, `pollSeconds`, and a `fetch` helper aimed at
this plugin's `/proxy/` path (backend Plugins only). Return a cleanup
function to release timers or listeners.

Styling is the design Token contract: [`docs/PLUGINS.md`](../../../docs/PLUGINS.md)
lists the CSS variables a Widget may use and the rules that keep it looking
native. Utility classes do not work here — Tailwind cannot see a module that
is loaded at runtime.
