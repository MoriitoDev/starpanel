# 02: Tailwind and the token layer

**What to build:** Tailwind v4 wired into `apps/web` with one stylesheet carrying every `DESIGN.md` value, Geist Sans self-hosted, and the identity mark as favicon — infrastructure only, no component redrawn yet.

**Blocked by:** 01 design-system-doc.

**Status:** done

- [x] `tailwindcss` and `@tailwindcss/vite` installed and wired into `vite.config.ts`
- [x] `apps/web/src/app.css` imported once from `main.ts`, holding `@font-face`, the `:root` light defaults and the `@theme inline` mapping of every token to its runtime variable
- [x] Geist Sans variable plus its OFL licence under `apps/web/public/fonts/`, preloaded in `index.html`, falling back to `system-ui`
- [x] `index.html` gains `lang="en"`, the four-point-star favicon and a `theme-color` per mode
- [x] `pnpm build` and `pnpm check` pass, and the existing UI still renders unchanged — this ticket adds the layer without consuming it

## Comments

- 2026-09-10: Tailwind v4.3.3 was installed by the owner. It sits in `dependencies` rather than `devDependencies`; moving it would desync `pnpm-lock.yaml` without network, so it stays put.
- 2026-09-10: `source(none)` plus explicit `@source` directives replaced automatic detection. The repo ships no `.gitignore`, so the scanner picked up `dist/assets/*.css` and re-emitted utilities from a probe that had already been deleted. Verified, fixed, and the built CSS went back from 13.15 kB to 10.41 kB.
- 2026-09-10: Token mapping verified by compiling a throwaway probe of every utility family and reading the emitted CSS: `bg-canvas` → `var(--canvas)`, `text-accent` → `var(--accent)`, `border-border` → `var(--border)`, `shadow-popover` → `var(--popover-shadow)`, and the text steps carry size, line-height, weight and tracking. The probe was deleted and the build re-run clean.
- 2026-09-10: Four additions beyond the letter of the ticket, all recorded in `DESIGN.md` §8: the default Tailwind palette is switched off (`--color-*: initial`); `:root` also carries the dark defaults behind `prefers-color-scheme`, so the shell does not flash light before the Dashboard loads; `--popover-shadow` is declared as a system value; and `src/vite-env.d.ts` was added so TypeScript accepts the stylesheet import. The font ships as the full charset (68 KB) rather than a subset.
- 2026-09-10: Checks: `pnpm build` builds clean, `pnpm check` reports 0 errors and 0 warnings, `pnpm test` passes 4/4, `go test ./...` passes.
