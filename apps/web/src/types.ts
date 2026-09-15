export interface Widget {
  id: string;
  plugin: string;
  /** Which of the plugin's widgets this entry renders; first when empty. */
  widget?: string;
  /**
   * The Span (ADR-0008): columns of the grid, 1-12, and rows, each a
   * `--row-height` tall that the card treats as a minimum. The Widget list is
   * ordered, so a Span says how big a card is and never where it sits.
   */
  w: number;
  h: number;
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
  /** Why this Plugin cannot run here, in words; absent when nothing is wrong. */
  problem?: string;
}

export interface PluginError {
  folder: string;
  error: string;
}

export interface PluginList {
  plugins: PluginInfo[];
  errors: PluginError[];
}

/** One Theme the panel knows: the default, or a stylesheet in themes/. */
export interface ThemeInfo {
  name: string;
  slug: string;
  /** False when an active Theme's file has gone missing from the folder. */
  present: boolean;
}

export interface ThemeList {
  active: string;
  themes: ThemeInfo[];
}

/** Context handed to a widget module's default export. */
export interface WidgetContext {
  config: Record<string, unknown>;
  pollSeconds: number;
  fetch: (path: string) => Promise<Response>;
}
