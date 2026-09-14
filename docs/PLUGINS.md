# Plugin widgets

How a Widget module looks so it belongs on the panel. This is the styling half of the Plugin contract: Manifests and backends are described in the repo [README](../README.md), and the visual language itself is [DESIGN.md](../DESIGN.md).

## The module

A Widget is framework-free ESM served as-is — no build step, no bundler, no Svelte:

```js
export default function render(el, ctx) {
  // paint into el
  return () => {}; // optional cleanup
}
```

`ctx` carries `config` (free-form JSON from the Dashboard), `pollSeconds`, and `fetch`, which targets this Plugin's `/proxy/` path (backend Plugins only). Return a cleanup function to release timers and listeners.

## The Manifest

```json
{
  "name": "my-plugin",
  "version": "0.1.0",
  "widgets": [{ "id": "my-widget", "title": "My Widget", "module": "widget.js" }],
  "backend": { "command": ["node", "backend.mjs"] },
  "requires": [{ "command": "node", "minVersion": "20" }]
}
```

The folder name must equal `name`, and every widget's `module` is a path inside it. `backend` and `requires` are optional.

## Backends

A backend is **a program core starts as a subprocess**, in any language, as long as it speaks HTTP on localhost:

- `command` is an argv array, run with the Plugin's folder as its working directory.
- Core allocates a free port and hands it over twice: `{port}` is replaced in any argument, and it arrives as `STAR_PANEL_PORT`.
- The backend must listen on `127.0.0.1`, not on every interface.
- It sees its own paths: a request to `/api/v1/plugins/my-plugin/proxy/items` reaches it as `/items`.
- Answer with JSON. If it dies, core restarts it every two seconds and the Widget reports the outage; the panel keeps working either way.

## What a Plugin needs to run

`requires` is the honest way to say what has to exist on the machine. Each entry names a command and, optionally, the version you had in mind.

```json
"requires": [{ "command": "node", "minVersion": "20" }]
```

A bare name is looked up in the `PATH`; a path with a separator — `./backend` — is looked for inside your Plugin's folder. The panel reports what is missing in the Plugin listing, before anyone adds the Widget, and shows the version you asked for without judging the one installed: comparing versions means parsing them, and every tool writes them differently, so the person reading decides.

There are two ways to satisfy a requirement, and both are legitimate:

**The runtime is on the machine.** `node`, `python3`, `java`. Simple, and it leaves your Plugin a couple of files.

**The Plugin brings its own.** A path like `./backend` points at a binary inside your folder, so the machine needs nothing. Rust, Go and Zig give you a static binary for free; Node can with `bun build --compile` or `deno compile`; Java needs `jlink`, which trims a runtime to about 40 MB. A compiled binary is per-platform: ship one per platform you support, or a small wrapper that picks.

**The panel diagnoses; it never installs.** It has no authentication by design, it runs without privileges, and a manifest that could name an installer would let a Plugin decide what runs on your server. So the panel says what is missing and you decide: install it, or, if you run Star Panel in Docker, add it to the image and rebuild.

## Styling: the Token contract

Widgets are loaded at runtime, so Tailwind's build-time scanner never sees them and **utility classes do not work**. The Dashboard instead sets the design Tokens as CSS variables on the shell around every card, so a Widget styles itself against the same values as the chrome and follows whatever Theme the owner has imported for free.

| Group | Variables |
| --- | --- |
| Colours | `--canvas` `--surface` `--surface-soft` `--border` `--border-soft` `--ink` `--body` `--mute` `--accent` `--accent-press` `--on-accent` `--danger` `--success` `--focus-ring` |
| Radii | `--radius-sm` (8px), `--radius-md` (12px), `--radius-lg` (16px), `--radius-pill` |
| Type | `--text-display`, `--text-heading`, `--text-subheading`, `--text-base`, `--text-meta`, each with its own `--text-<step>--line-height` |
| Motion | `--ease-quiet`, `--default-transition-duration` |

```js
value.style.cssText =
  "color:var(--ink);font-size:var(--text-meta);font-variant-numeric:tabular-nums;";
```

### Rules

- **Never a literal colour.** Every colour comes from a variable; that is what keeps a Widget readable under a Theme we have never seen.
- **At most one accent per Widget.** `--success` and `--danger` report state, `--accent` is for the one thing that leads or acts.
- **No shadows, no gradients, no second accent.** The system allows exactly one shadow, on popovers, and a Widget is not a popover.
- **Numbers use tabular figures:** `font-variant-numeric: tabular-nums`.
- **The card already supplies the frame.** A Widget paints inside `el`; the title, the border, the surface and the padding belong to the Dashboard.
- **A failing Widget must not break the panel.** Render a message in `--danger` rather than throwing, and release timers in the returned cleanup function.

## Reference

[`hello-widget`](../apps/api/plugins/hello-widget/README.md) is the smallest complete example, and `system-stats-custom` is the same shape with a backend behind it.

## Publishing one

There is no registry: a Plugin is a folder. Publish yours as a repository with the folder inside, or as a ZIP of it, and a README that says what it needs — `requires` says it too.

## Installing, and taking one out

The short road is the folder itself: copy it into `plugins/` on the machine running Star Panel, and the panel picks it up on its next look — no restart, no refresh. Deleting the folder uninstalls it.

In edit mode, **Import a Plugin…** takes a ZIP instead, for when the panel is not where your hands are. Zip the folder, or zip its contents; both arrive at the same Plugin, because the Manifest inside decides which. The archive is unpacked into `plugins/` under the name its Manifest declares, and every step is checked on the way:

- A Manifest that does not parse, a widget with no module, a path that climbs out of the folder, or a name already in `plugins/` leaves **nothing** behind — not a half-written folder, and not the archive, which is never written to disk.
- An existing Plugin is never overwritten: the import is refused and the message names the folder in the way. Deleting the old one is your move.
- A `requires` this machine cannot satisfy is reported in the answer to the import, before anyone adds the Widget.

**Download** next to a Plugin in that section saves its folder as a ZIP, the same shape an import takes back — handy for moving one between machines.

Importing a Plugin is importing code: its widgets run in the page and its backend runs with the panel's privileges, and Star Panel has no login ([ADR-0001](adr/0001-single-user-no-auth.md)). Anyone who can reach the port can install one, so keep the panel on a network you trust ([ADR-0007](adr/0007-plugin-import-is-an-upload-of-code.md)).
