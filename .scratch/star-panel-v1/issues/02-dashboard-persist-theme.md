# 02: Dashboard persist plus Theme

**What to build:** Dashboard ordered list with enable toggle plus light/dark Theme that survives restart, proving flat-document persistence end to end.

**Blocked by:** 01 monorepo-scaffold-health.

**Status:** done

- [x] Dashboard get plus save round-trips ordered Widgets with size and config plus Theme vars
- [x] Dashboard UI reorders, toggles, and switches Theme with 10s poll default visible
- [x] Restart preserves layout plus enabled set plus Theme
- [x] HTTP boundary tests cover get/save validation and persistence

## Comments

- 2026-09-08: Implemented. GET/PUT /api/v1/dashboard with validation (widget ids, sizes, theme mode/colors, pollSeconds defaulting to 10), atomic flat-JSON store in -data-dir (default ./data/dashboard.json), default seeded layout with two sample widgets. Web UI: reorder buttons, enable toggles, light/dark theme switch driving CSS variables, per-widget poll display, 10s dashboard re-poll, auto-save on change. Boundary tests cover round-trip, validation rejections, malformed JSON, and restart persistence. OpenAPI updated with dashboard schemas.

- 2026-09-08 (fix): Theme switch had no visual effect — the document stored a single color set, so flipping mode changed nothing on screen. Theme now stores a palette per mode (light/dark, each background/foreground/accent); mode selects which one renders. Legacy flat-theme documents migrate on load (saved colors move to the palette of the stored mode, missing palettes get defaults). Verified in the running app: dark->light->dark switches recolor the dashboard and persist.
