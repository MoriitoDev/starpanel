import type { Dashboard, PluginList } from "./types";

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
