# 03: Frontend Plugin contract plus hello-widget

**What to build:** folder-drop Plugin with Manifest validation plus framework-free Widget rendering, proven by a hello-widget reference users can copy.

**Blocked by:** 01 monorepo-scaffold-health, 02 dashboard-persist-theme.

**Status:** done

- [x] Dropping a Plugin folder with Manifest makes it appear in Plugin listing after validation
- [x] Dashboard renders hello-widget via vanilla ESM with config plus Theme vars plus poll
- [x] Invalid Manifest is rejected with a clear error and never breaks the Dashboard
- [x] Contract tests cover Manifest parse/validate plus widget load/render with stub context

## Comments

- 2026-09-08: Implemented per ADR-0003. Plugin folders (apps/api/plugins/) with manifest.json are discovered on every listing request: GET /api/v1/plugins returns valid plugins plus rejected folders with clear errors (invalid folders never break the listing or dashboard). Module files served at GET /api/v1/plugins/{name}/modules/{rest...} with traversal protection. Dashboard mounts widget modules as framework-free ESM (WidgetCard.svelte): default export render(el, ctx) with ctx = {config, theme, pollSeconds, fetch->proxy}; returned cleanup runs on reorder/unmount; effect tracks primitives only so the 10s re-sync does not reset widgets. hello-widget reference plugin (manifest + widget.js + README) renders config message with Theme vars and poll tick. Dashboard Widget model gained optional `widget` field; legacy sample widgets migrate to hello-widget. Tests: Go manifest parse/validate + listing + module serving; JS contract tests (node --test, pnpm test) cover manifest shape and widget render with stub element/context.
