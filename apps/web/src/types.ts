export type WidgetSize = "small" | "medium" | "large";

export interface Widget {
  id: string;
  plugin: string;
  /** Which of the plugin's widgets this entry renders; first when empty. */
  widget?: string;
  size: WidgetSize;
  enabled: boolean;
  pollSeconds: number;
  config?: Record<string, unknown>;
}

export interface Dashboard {
  widgets: Widget[];
  /** The name of the imported Theme to render with; "default" is the baseline. */
  theme: string;
}

export interface PluginWidgetSpec {
  id: string;
  title: string;
  /** API URL to import the widget's ESM module from. */
  module: string;
}

export interface PluginInfo {
  name: string;
  version: string;
  widgets: PluginWidgetSpec[];
  backend: boolean;
}

export interface PluginError {
  folder: string;
  error: string;
}

export interface PluginList {
  plugins: PluginInfo[];
  errors: PluginError[];
}

/** Context handed to a widget module's default export. */
export interface WidgetContext {
  config: Record<string, unknown>;
  pollSeconds: number;
  fetch: (path: string) => Promise<Response>;
}
