# 04: Backend Plugin subprocess plus proxy

**What to build:** optional backend command per Manifest supervised by core and reachable via per-Plugin proxy path, proving any-language backends without core recompile.

**Blocked by:** 01 monorepo-scaffold-health, 03 frontend-plugin-hello-widget.

**Status:** done

- [x] Manifest backend command starts/stops with core and proxy path forwards to subprocess HTTP
- [x] Subprocess crash maps to a Widget-friendly error without dropping the Dashboard
- [x] Pure-frontend Plugins keep working with no backend declared
- [x] HTTP boundary tests cover proxy passthrough plus stopped-plugin plus error mapping

## Comments

- 2026-09-08: Implemented per ADR-0003. Supervisor (apps/api/backend.go) starts each valid plugin's backend command at core startup (plus lazy start on first proxy hit) on an allocated 127.0.0.1 port passed via STAR_PANEL_PORT env and {port} arg substitution, cwd = plugin folder; restarts with 2s backoff until core shuts down (signal context, StopAll). Proxy at /api/v1/plugins/{name}/proxy/{rest...} forwards all methods with the prefix stripped. Errors map widget-friendly: crashed/stopped backend or lazy-start timeout -> 502 {error}, frontend-only plugin -> 502 "has no backend", unknown plugin -> 404; dashboard endpoints unaffected (asserted). Pure-frontend plugins keep working with no backend declared. Boundary tests use the re-executed test binary as a stand-in subprocess (passthrough incl. backend 404 mapping, crash -> 502, frontend-only -> 502). Shipped echo demo plugin (node backend.mjs) proving any-language backends; verified live: proxy ping 200, widget shows "backend alive" via ctx.fetch polling.
