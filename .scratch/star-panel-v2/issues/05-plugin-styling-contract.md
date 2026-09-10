# 05: Plugin styling contract and bundled widgets

**What to build:** the widget styling contract written for Plugin authors, and the three bundled widgets restyled against it in English.

**Blocked by:** 02 tailwind-token-layer.

**Status:** done

- [x] `docs/PLUGINS.md` states which tokens a widget may use (fourteen colors, four radii, the type steps), how to consume them (`var(--accent)`), and what widgets must not do: utility classes, literal colors, shadows, a second accent
- [x] `system-stats`, `hello-widget` and `echo` are restyled against the tokens with no literal color left, and their copy is English
- [x] `system-stats` renders its values with tabular figures and looks native to the panel
- [x] `hello-widget/README.md` and the widget modules' header comments point at `docs/PLUGINS.md`
- [x] `pnpm test` still passes: every bundled widget renders with a stub element and stub context

## Comments

- 2026-09-10: The widget contract is unchanged (ADR-0003); only the token names and the copy moved. `echo` now reports state through the tokens that mean state — `--success` when the backend answers, `--danger` when it does not — instead of the old accent-for-everything.
- 2026-09-10: One judgement call: `system-stats` fills its bars with `--ink` rather than `--accent`, because `DESIGN.md` reserves the accent for the one thing that acts or leads, and a monochrome bar keeps the accent meaning something. Easy to revisit if the bars read as dividers.
- 2026-09-10: The contract tests constrained the rewrite usefully: the stub element supports only `append`, `replaceChildren`, `textContent` and `style.cssText`, so the widgets stay inside that surface, and `hello-widget` keeps its `poll every Ns` line.
