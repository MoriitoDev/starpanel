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

/** One mode's set of CSS variables: the fourteen DESIGN.md tokens. */
export interface Palette {
  canvas: string;
  surface: string;
  surfaceSoft: string;
  border: string;
  borderSoft: string;
  ink: string;
  body: string;
  mute: string;
  accent: string;
  accentPress: string;
  onAccent: string;
  danger: string;
  success: string;
  focusRing: string;
}

export interface Theme {
  /** "auto" follows the operating system; light and dark are explicit. */
  mode: "light" | "dark" | "auto";
  light: Palette;
  dark: Palette;
}

export interface Dashboard {
  widgets: Widget[];
  theme: Theme;
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
  theme: Palette;
  pollSeconds: number;
  fetch: (path: string) => Promise<Response>;
}
