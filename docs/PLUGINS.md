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

`ctx` carries `config` (free-form JSON from the Dashboard), `theme` (the active Palette as data), `pollSeconds`, and `fetch`, which targets this Plugin's `/proxy/` path (backend Plugins only). Return a cleanup function to release timers and listeners.

## Styling: the Token contract

Widgets are loaded at runtime, so Tailwind's build-time scanner never sees them and **utility classes do not work**. The Dashboard instead sets the design Tokens as CSS variables on the shell around every card, so a Widget styles itself against the same values as the chrome and follows light, dark and auto for free.

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

- **Never a literal colour.** Every colour comes from a variable; that is what keeps a Widget readable when the owner switches between light, dark and auto.
- **At most one accent per Widget.** `--success` and `--danger` report state, `--accent` is for the one thing that leads or acts.
- **No shadows, no gradients, no second accent.** The system allows exactly one shadow, on popovers, and a Widget is not a popover.
- **Numbers use tabular figures:** `font-variant-numeric: tabular-nums`.
- **The card already supplies the frame.** A Widget paints inside `el`; the title, the border, the surface and the padding belong to the Dashboard.
- **A failing Widget must not break the panel.** Render a message in `--danger` rather than throwing, and release timers in the returned cleanup function.

## Reference

[`hello-widget`](../apps/api/plugins/hello-widget/README.md) is the smallest complete example, and `system-stats` is the same shape with a backend behind it.
