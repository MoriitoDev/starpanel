import type { Dashboard, PluginInfo, PluginList, ThemeInfo, ThemeList } from "./types";

async function parse<T>(res: Response): Promise<T> {
  const body = await res.json();
  if (!res.ok) {
    const message = typeof body.error === "string" ? body.error : res.statusText;
    throw new Error(message);
  }
  return body as T;
}

export function fetchDashboard(): Promise<Dashboard> {
  return fetch("/api/v1/dashboard").then((res) => parse<Dashboard>(res));
}

export function saveDashboard(dashboard: Dashboard): Promise<Dashboard> {
  return fetch("/api/v1/dashboard", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(dashboard)
  }).then((res) => parse<Dashboard>(res));
}

export function fetchPlugins(): Promise<PluginList> {
  return fetch("/api/v1/plugins").then((res) => parse<PluginList>(res));
}

/** A Plugin arrives as a ZIP of its folder; the name comes from its Manifest. */
export function importPlugin(archive: Blob): Promise<PluginInfo> {
  return fetch("/api/v1/plugins", {
    method: "POST",
    headers: { "Content-Type": "application/zip" },
    body: archive
  }).then((res) => parse<PluginInfo>(res));
}

/** Where the browser fetches a Plugin's folder archive from, for a download. */
export function pluginArchiveHref(name: string): string {
  return `/api/v1/plugins/${encodeURIComponent(name)}/archive`;
}

export function fetchThemes(): Promise<ThemeList> {
  return fetch("/api/v1/themes").then((res) => parse<ThemeList>(res));
}

/** The stylesheet itself is the request body; the name comes from its header. */
export function importTheme(css: string): Promise<ThemeInfo> {
  return fetch("/api/v1/themes", {
    method: "POST",
    headers: { "Content-Type": "text/css" },
    body: css
  }).then((res) => parse<ThemeInfo>(res));
}

export function deleteTheme(slug: string): Promise<void> {
  return fetch(`/api/v1/themes/${encodeURIComponent(slug)}`, { method: "DELETE" }).then(
    async (res) => {
      if (res.ok) return;
      const body = await res.json().catch(() => ({}));
      throw new Error(typeof body.error === "string" ? body.error : res.statusText);
    }
  );
}

/** Where the browser fetches a Theme's stylesheet from, for a link or a download. */
export function themeHref(slug: string): string {
  return `/api/v1/themes/${encodeURIComponent(slug)}.css`;
}
