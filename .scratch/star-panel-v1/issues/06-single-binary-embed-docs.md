# 06: Single-binary embed plus docs

**What to build:** one Go binary serving embedded web build plus API with beside-binary config plus Plugins folders, proving lightweight prod and documented dev.

**Blocked by:** 01 monorepo-scaffold-health, 02 dashboard-persist-theme, 03 frontend-plugin-hello-widget, 04 backend-plugin-subprocess-proxy, 05 system-stats-plugin.

**Status:** done

- [x] Single binary serves Dashboard plus v1 API on one port with no Node runtime required
- [x] Fresh checkout runs via two documented dev commands and produces the binary via one build command
- [x] Config plus Plugins folders resolve beside the binary with a working default Dashboard
- [x] Final verify covers cold-start binary plus enable-toggle plus restart persistence

## Comments

- 2026-09-10: The Dashboard build lands in `apps/api/webdist` instead of `apps/web/dist`, because `//go:embed` cannot reach above its own package directory. `apps/web`'s Vite config points there directly, so there is no copy step and no stale-copy failure mode. The folder ships a placeholder, so a checkout that never built the front still compiles and answers with a plain explanation instead of an empty page.
- 2026-09-10: Routing rules for the embedded build are tested at the HTTP seam with a fake filesystem, not the compiled-in one, so the tests hold whatever the build happens to contain: the shell at `/`, real assets served with their content type, an unrouted path falling back to the shell, a *missing* asset answering 404 rather than HTML (a browser would otherwise parse the shell as a module), unrouted `/api/` paths staying JSON, and a build-less binary explaining itself.
- 2026-09-10: `data/` and `plugins/` now resolve beside the binary when the flags are left alone, which is what a deployed Star Panel needs; naming them explicitly keeps them relative to the working directory, which is what `go run` needs, since it builds its binary in a temporary folder. The dev commands therefore pass both flags on purpose, and startup logs the resolved absolute paths.
- 2026-09-10: The module was renamed from `star-panel/api` to `star-panel` (nothing imports it) so Go names its own output: `go build -C apps/api .` produces `star-panel` on Linux and `star-panel.exe` on Windows. With an explicit `-o star-panel` the Windows binary had no extension and the shell refused to run it, which is exactly the kind of thing the cold-start check exists to catch.
- 2026-09-10: One build command at the repo root (`pnpm build`): web build, then the binary. Verified end to end by copying the binary plus its `plugins/` folder into an empty directory: cold start served the shell, the hashed asset and all three plugins; the enable toggle wrote `data/dashboard.json`; and a process restart brought the same Dashboard back, both through the API and in the browser.
