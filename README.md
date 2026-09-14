# Star Panel

Lightweight self-hosted homelab dashboard: a single Go binary serves the static frontend (Vite + Svelte) and a versioned REST API. No auth (trusted network), no database (flat JSON), no Node runtime in prod.

## How it works

- **Backend (`apps/api`, Go):** one process serves the API and the embedded frontend on the same port. It stores the Dashboard in `apps/api/data/dashboard.json` (ordered Widget list + the active Theme's name). It discovers Plugins by folder in `apps/api/plugins/<name>/manifest.json` and Themes by stylesheet in `apps/api/themes/`. Plugin backends run as supervised localhost subprocesses, proxied via `/api/v1/plugins/{name}/proxy/...`. Ships with a built-in `system-stats` plugin.
- **Frontend (`apps/web`, Vite + Svelte SPA, no SSR):** reads `GET /api/v1/dashboard`, renders each Widget by importing its ESM module from `/api/v1/plugins/{name}/modules/...`, and saves with `PUT /api/v1/dashboard`. Modules are vanilla ESM: `export default function render(el, ctx)` with `ctx = { config, pollSeconds, fetch }`. Plain REST polling every 10s (per-Widget default). View mode shows the data alone; the header's Edit toggle reveals the add control and each Widget's remove, enable and move controls.
- **Plugin =** a folder with `manifest.json` (name, version, widgets[], optional backend) + widget files. Adding one means dropping its folder into `plugins/` or importing a ZIP of it from edit mode, which unpacks it in the same place; removing one means deleting the folder. `GET /api/v1/plugins` lists valid ones + rejected folders with a clear error (never breaks the Dashboard), and a Plugin that needs something this machine does not have says so in the listing and in the answer to its import — see [ADR-0007](docs/adr/0007-plugin-import-is-an-upload-of-code.md) for why an import carries a warning.
- **Backend layout:** `main` wires it, `server.go` is the whole HTTP surface behind one `newServer(deps)`, and the two domains live in `internal/dashboard` (the Dashboard document and its store) and `internal/plugins` (Manifest, folder discovery, supervised backends) — see [ADR-0005](docs/adr/0005-go-backend-modules.md).
- **Theme:** the Dashboard stores the *name* of the Theme it renders with. `default` is the baseline palette inside `apps/web/src/app.css` and always available; anything else is a CSS file imported into `themes/`, layered on top — see [ADR-0006](docs/adr/0006-themes-are-stylesheets.md). Styling is Tailwind v4 over that baseline, and Plugin Widgets style themselves against the same Tokens as CSS variables ([docs/PLUGINS.md](docs/PLUGINS.md)). Geist Sans, self-hosted; the four-point star is the only logo asset.

## Requirements

- Go 1.27+ (back), Node + pnpm (front dev).

## Startup (2 terminals, dev)

```powershell
# Terminal 1 — back (from apps/api, API on :8080)
cd apps/api
go run . -data-dir data -plugins-dir plugins -themes-dir themes

# Terminal 2 — front (from apps/web, dev on :5173 with /api/v1 proxy -> :8080)
cd apps/web
pnpm install
pnpm dev
```

Open http://localhost:5173 (back listens on http://localhost:8080).

Those flags are not decoration: left alone, `data/`, `plugins/` and `themes/` resolve beside the binary — where a deployed Star Panel keeps them — and `go run` builds its binary in a temporary folder.

## Build and run the binary

```powershell
# from the repo root: builds the web into apps/api/webdist, then the binary
pnpm build

# Dashboard plus API on http://localhost:8080, no Node runtime involved.
# The module is named after the product, so Go names the binary itself:
# apps/api/star-panel on Linux and macOS, apps/api/star-panel.exe on Windows.
./apps/api/star-panel
```

Those folders resolve beside the binary, so a deployment is the binary plus its `plugins/` folder, and its `themes/` one if it has imported any:

```powershell
mkdir C:\star-panel
copy apps\api\star-panel, apps\api\plugins C:\star-panel -Recurse
C:\star-panel\star-panel -addr :8080
```

A binary built without the web build still runs: it serves the API and explains that the Dashboard is missing.

## Run it in Docker

```powershell
# from the repo root: builds the Dashboard, the binary, and the image
docker compose up -d --build
```

Then http://localhost:8080. The Dashboard and the imported Themes live in named volumes, so they survive a rebuild; to drop a Theme in by hand, `docker cp midnight.css star-panel:/app/themes/`.

Three things worth knowing before you put this on a server:

- **Stats are the container's view.** `system-stats` reads `/proc` and the root filesystem, so memory and CPU are roughly the host's, but **disk is the container's**, not your server's. A panel that reports the wrong disk is worse than one that reports none, so treat the disk row as decoration until the stats source can be pointed at a host mount.
- **A Plugin backend needs its runtime in the image.** The bundled `system-stats` is core, `hello-widget` has no backend, and `echo` is left out on purpose: its backend is a Node script and the runtime image has no Node.
- **A Plugin that talks to Docker needs the socket.** Mount `/var/run/docker.sock` and make sure the container's user may use it.

## Commands

```powershell
# Front
cd apps/web
pnpm dev    # dev server on :5173
pnpm build  # build into apps/api/webdist (embedded by the Go binary)
pnpm check  # svelte-check
pnpm test   # plugin contract tests (node --test)

# Root
pnpm build  # web build + Go binary in one command

# Back
cd apps/api
go run . -data-dir data -plugins-dir plugins -themes-dir themes   # serve
go test ./...   # HTTP tests (health, stats, plugins, dashboard, proxy)
```

## Endpoints (v1)

- `GET /api/v1/health` — liveness + version.
- `GET / PUT /api/v1/dashboard` — get/save the Dashboard.
- `GET /api/v1/plugins` — `{ plugins[], errors[] }`.
- `POST /api/v1/plugins` — imports a Plugin: a ZIP of its folder is the request body, and the name comes from the Manifest inside.
- `GET /api/v1/plugins/{name}/archive` — the Plugin's folder as a ZIP, which is what the download button points at.
- `GET /api/v1/plugins/{name}/modules/{rest...}` — serves the widget ESM.
- `/{name}/proxy/{rest...}` — proxies to the plugin backend (or built-in).
- `GET /api/v1/themes` — `{ active, themes[] }`, the default first.
- `POST /api/v1/themes` — imports a Theme: the stylesheet is the request body.
- `GET /api/v1/themes/{name}.css` — serves a Theme, and is what the download button points at.
- `DELETE /api/v1/themes/{name}` — removes an imported Theme.
