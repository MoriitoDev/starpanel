# Themes

A Theme is a stylesheet. It repaints the panel — colours, borders, hovers, spacing, whatever it likes — and the panel does not check it. This is what you can count on when you write one.

## Where Themes live

`themes/` beside the binary, one `.css` file per Theme. The default is not a file: it is the baseline in `apps/web/src/app.css`, always first in the list and never deletable. Importing writes `themes/<slug>.css`; you can also drop a file into that folder by hand, and the panel picks it up on its next look.

## The file

An optional header comment names the Theme.

```css
/* @name: Midnight */
:root {
  --canvas: #0e1012;
  --surface: #17191c;
  --ink: #f3f4f6;
  --accent: #7c8dff;
  color-scheme: dark;
}
```

Without `@name`, an import is named `theme-1`, `theme-2` and so on; a file dropped into the folder keeps its file name. `@author` and `@version` are read by nobody today.

## What a Theme can rely on

**The Token variables.** The fourteen colours, the four radii, the type steps, the motion values and the popover shadow — [DESIGN.md](../DESIGN.md) §2 lists them. The default sets them in `:root`; a Theme overrides whichever it wants and inherits the rest, so a three-line Theme is a legitimate Theme.

**The component classes.** `.btn`, `.btn-primary`, `.btn-secondary`, `.btn-ghost`, `.control`.

**Six region hooks.** `data-part="app"` (the page itself), `"header"`, `"identity"`, `"card"`, `"empty"` and `"error"`.

That is the whole promise. Anything else you write against our markup is allowed and **unsupported**: it works until we tidy a class or a `div`, and then it does not. If you need a hook we do not have, ask for it rather than reaching into the markup.

## Things to change, and where

**The page itself.** `data-part="app"` is the root element, so this is where a wallpaper goes:

```css
[data-part="app"] {
  background-image: url("https://example.com/wallpaper.jpg");
  background-size: cover;
  background-position: center;
  background-repeat: no-repeat;
}
```

Cards sit on top of it with an opaque `--surface`, so the image shows between and around them. Make `--surface` translucent if you want it behind them too, and remember that text over a picture costs contrast — your call to make, not the panel's.

**Corners.** The four radii are variables, so squaring the panel off is one rule:

```css
:root { --radius-sm: 2px; --radius-md: 2px; --radius-lg: 2px; --radius-pill: 2px; }
```

**Spacing.** `--spacing` is the unit every spacing utility multiplies, so changing it changes all of them at once — blunt, and occasionally exactly what you want:

```css
:root { --spacing: 0.2rem; }   /* tighter everywhere */
```

For one region rather than everywhere, use its hook: `[data-part="card"] { padding: 2rem; }`.

**Type.** The family and the steps are variables too:

```css
:root {
  --font-sans: "Iowan Old Style", Georgia, serif;
  --text-base: 17px;
  --text-base--line-height: 27px;
}
```

**The chrome.** `.btn`, `.btn-primary`, `.btn-secondary`, `.btn-ghost` and `.control` are ours and staying; a Theme may restyle any of them.

**On specificity.** Your stylesheet is linked after ours, so a rule of yours that is equally specific wins without `!important`. When you find yourself reaching for `!important`, that is usually a sign you are styling something outside the contract.

## Rules

- Never assume your Theme is the only one: another one may be painting the same markup.
- `prefers-color-scheme` is yours. A Theme that follows the system says so in its own CSS, with the media query.
- A Theme may load remote resources — a webfont, a background image. What the panel promises is that *it* works offline; what you install is your call ([ADR-0006](./adr/0006-themes-are-stylesheets.md)).
- Widgets style themselves with the same variables, so repainting the panel repaints every Widget with it.

## Importing, deleting, downloading

In edit mode, **Import a Theme…** reads a `.css` file and sends it; the name comes from its header. A name that is already imported is refused rather than overwritten — delete the old one first. Deleting asks for confirmation, and deleting the active Theme sends the panel back to its default. **Download** saves the active Theme's file and is disabled while the default is active, because the default is not a file.
