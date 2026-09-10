Status: ready-for-agent

## Problem Statement

As a homelab owner, I want one lightweight Dashboard I can shape to my setup, so that I see only the Widgets I care about with a Theme I like without running heavy infrastructure.

## Solution

A single Go binary serves a static Vite + Svelte Dashboard and a versioned REST API. A Plugin supplies one or more Widgets described by a Manifest. Users add or remove Plugins by folder drop plus toggle, create pure-frontend or backend-backed Plugins, and switch Theme variables. V1 proves this with system-stats plus a hello-widget reference.

## User Stories

1. As a homelab owner, I want to view my Dashboard as an ordered list of Widgets, so that I see everything at a glance.
2. As a homelab owner, I want to enable or disable a Plugin via toggle, so that my Dashboard stays unique to me.
3. As a homelab owner, I want to add a Plugin by dropping a folder with a Manifest, so that I need no marketplace or upload UI.
4. As a Plugin author, I want to build a frontend-only Plugin in vanilla ESM plus CSS vars, so that I need no Svelte toolchain.
5. As a Plugin author, I want to build a backend-backed Plugin as a subprocess speaking HTTP on localhost, so that I can use any language without recompiling core.
6. As a homelab owner, I want to see system CPU, memory, and disk from a built-in Plugin, so that base monitoring works out of the box.
7. As a Plugin author, I want a hello-widget reference Plugin, so that I can copy the minimal Manifest plus widget pattern.
8. As a homelab owner, I want to switch Theme between light and dark plus accent, so that the panel fits my taste.
9. As a homelab owner, I want my Dashboard layout plus enabled Plugins plus Theme persisted across restarts, so that I configure once.
10. As a homelab owner, I want Widgets to poll on a 10s default, so that data stays fresh without complex realtime code.
11. As a homelab owner, I want to run everything as one low-idle binary with no Node runtime in prod, so that old PCs to high-end labs all cope.
12. As a developer, I want two-command dev (Go API plus Vite dev with API proxy), so that I can iterate without orchestration tools.
13. As a developer, I want shared API types from a single OpenAPI source, so that frontend and backend stay in sync.
14. As a homelab owner, I want health and version endpoints, so that I can tell the panel is alive.

## Implementation Decisions

- Modules: API backend in Go, web Dashboard in Vite plus Svelte SPA with no SSR, shared API contract package, plus two V1 Plugins (system-stats, hello-widget).
- Monorepo managed by pnpm workspaces covering web and shared areas; Go module versioned with Go 1.27 toolchain.
- Architecture: Go serves embedded static web build plus REST API from one process; no auth layer per ADR-0001; single-user trusted network assumed.
- API contract versioned under v1: health check, system stats read, Plugin listing, Dashboard get plus save, and per-Plugin proxy path forwarding to the Plugin backend subprocess.
- Dashboard model is an ordered list only for V1, each entry carrying identity, Plugin reference, size variant (small/medium/large), and free-form per-Widget config; no drag grid coordinates.
- Plugin model: Manifest declares name, version, provided Widgets, and optional backend command; frontend entry is a framework-free ESM module rendering into a provided element with a context (fetch helper, config, Theme vars); backend entry is a supervised subprocess exposing localhost HTTP, proxied by core.
- Plugin discovery is folder drop plus toggle; enabled set plus layout plus Theme live in a single flat JSON document; no database in V1.
- Theme is CSS variables only (background, foreground, accent, light/dark mode) stored inside the Dashboard document; no custom CSS files in V1.
- System-stats uses stdlib-only Linux proc parsing with a stub on other OSes for dev; no system-info dependency in V1.
- Polling is plain REST polling at 10s default, configurable per Widget; no websockets or server-sent events in V1.
- Distribution is a single binary with embedded web build; config plus Plugins live beside the binary as folders; container image optional and not required.
- Dev workflow is two terminals with the web dev server proxying API calls to the Go port; no monorepo task runner in V1.

## Testing Decisions

- A good test asserts external behavior at the seam, never internals: HTTP status plus body shape for the API, Manifest validity plus widget render output for Plugins.
- Tested at seam 1 (HTTP v1 boundary): health, system stats shape, Plugin list, Dashboard get/save round-trip, Plugin proxy passthrough and error mapping.
- Tested at seam 2 (Plugin contract): Manifest parses and validates, frontend widget module loads and renders with stub context, backend subprocess supervision starts/stops/proxies.
- Prior art: none, greenfield. Use Go httptest-style HTTP tests for the API and a lightweight JS test runner for the widget contract; no browser E2E in V1 beyond manual verify.

## Out of Scope

- Login, multi-user, permissions, or auth provider integration.
- History, graphs, or any database; SQLite arrives only with a history ticket.
- Drag-drop grid layout, resizing, or custom layouts per device.
- Marketplace, UI upload, URL install, signing, or sandboxing beyond OS user trust.
- Custom CSS files or per-Widget theming beyond shared variables.
- Websockets, server-sent events, logs tailing, or terminal access.
- Required Docker image, reverse-proxy config, TLS automation, or auto-updates.
- Windows/macOS production stats parity beyond dev stub; Linux is the V1 target.
- Monorepo orchestration (task runner, affected builds, shared CI matrix).

## Further Notes

- Respects ADR-0001 (no auth), ADR-0002 (Vite plus Svelte SPA over Astro/Next), ADR-0003 (subprocess plus vanilla ESM).
- Uses glossary terms Dashboard, Widget, Plugin, Manifest, Theme throughout.
- V1 proves plumbing plus two Plugins; every deferred item above becomes its own ticket only on demand.
