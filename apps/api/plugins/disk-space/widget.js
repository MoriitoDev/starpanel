// The disk-space card: every Disk with a bar, the space this Plugin could
// reclaim, and the button that opens the detail view.
//
// Framework-free ESM by contract (docs/PLUGINS.md): no build step, no bundler,
// and no utility class, because Tailwind's scanner never sees this file. Every
// colour, radius and type step comes from a Token, so the Widget follows
// whatever Theme the owner imported.

/** One decimal, no trailing .0, binary units — the card and the dialog share it. */
export function formatBytes(bytes) {
  if (bytes === null || bytes === undefined || Number.isNaN(bytes)) return "?";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  let value = Math.abs(bytes);
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  const rounded = Math.round(value * 10) / 10;
  const text = Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
  return `${text} ${units[unit]}`;
}

/** The warn threshold, read from the widget's config with the documented default. */
export function warnPercentFrom(config) {
  const raw = config && typeof config === "object" ? config.warnPercent : undefined;
  const value = typeof raw === "number" ? raw : Number.parseInt(raw, 10);
  if (!Number.isFinite(value) || value < 1 || value > 100) return 90;
  return value;
}

// One element per piece of the card, styled with cssText so nothing here needs
// a class the Dashboard does not have.
function element(tag, css, text) {
  const node = document.createElement(tag);
  if (css) node.style.cssText = css;
  if (text !== undefined) node.textContent = text;
  return node;
}

const META = "color:var(--mute);font-size:var(--text-meta);";
const BODY = "color:var(--body);font-size:var(--text-base);";
const NUMBER = "font-variant-numeric:tabular-nums;";

// A bar is a track in surface-soft with a fill in ink: the accent is for the
// one thing that acts, and on this card that is Clean….
function bar(percent) {
  const track = element(
    "div",
    "background:var(--surface-soft);border-radius:var(--radius-pill);height:6px;overflow:hidden;width:100%;"
  );
  const width = Math.max(0, Math.min(100, Number.isFinite(percent) ? percent : 0));
  track.append(
    element(
      "div",
      `background:var(--ink);height:100%;width:${width}%;transition:width var(--default-transition-duration) var(--ease-quiet);`
    )
  );
  return track;
}

// A Disk row: what it is called, how full it is, and what is left. The state is
// written down — "nearly full" — because DESIGN.md forbids communicating
// anything by colour alone.
export function diskRow(disk, warnPercent) {
  const row = element("div", "display:grid;gap:4px;padding:8px 0;");
  row.append(
    element("div", `${BODY}color:var(--ink);`, `${disk.name} · ${disk.label} · ${disk.fs}`)
  );
  row.append(bar(disk.usedPercent));
  const full = disk.usedPercent >= warnPercent;
  const summary = element(
    "div",
    `${META}${NUMBER}${full ? "color:var(--danger);" : ""}`,
    `${disk.usedPercent}% · ${formatBytes(disk.freeBytes)} free${full ? " · nearly full" : ""}`
  );
  row.append(summary);
  return row;
}

export default function render(el, ctx) {
  const config = ctx.config && typeof ctx.config === "object" ? ctx.config : {};
  const warnPercent = warnPercentFrom(config);
  const state = { timer: null, attempts: 0, stopped: false };

  function paint(children) {
    el.replaceChildren(...children);
  }

  function starting() {
    // Exactly one node: a second one would mean "this is the answer", and the
    // card would stop looking like it is still waiting.
    paint([element("p", META, "Reading the disks…")]);
  }

  function failed(message) {
    paint([element("p", `${BODY}color:var(--danger);`, `Could not read the disks: ${message}`)]);
  }

  function loaded(summary) {
    const children = [];
    const disks = Array.isArray(summary.disks) ? summary.disks : [];
    for (const disk of disks) children.push(diskRow(disk, warnPercent));

    if (disks.length === 0) {
      children.push(element("p", META, "No volumes found"));
    }

    const reclaimable = element(
      "div",
      `${BODY}color:var(--ink);${NUMBER}margin-top:8px;`,
      `${summary.reclaimableIsEstimate ? "≈ " : ""}${formatBytes(summary.reclaimableBytes)} reclaimable`
    );
    children.push(reclaimable);
    if (summary.reclaimableIsEstimate) {
      children.push(
        element("p", META, "An estimate: shared Docker layers are counted once by nobody.")
      );
    }

    // At most one muted line beyond the body (DESIGN.md §4). Config notes and
    // warnings are both that line, and a warning about an unreadable Scan Root
    // outranks a note about a default.
    const notes = Array.isArray(summary.warnings) ? summary.warnings.filter(Boolean) : [];
    if (summary.configWarning) notes.unshift(summary.configWarning);
    if (notes.length > 0) {
      const line = element("p", META, notes.join(" · "));
      line.setAttribute("title", notes.join("\n"));
      children.push(line);
    }

    if (disks.length > 0) {
      children.push(
        element(
          "p",
          META,
          `${disks.length} volume${disks.length === 1 ? "" : "s"} · source: ${summary.source}`
        )
      );
    }

    // The one control on the card, and the one accent on this surface.
    const clean = element(
      "button",
      "background:var(--accent);color:var(--on-accent);border:0;border-radius:var(--radius-sm);padding:6px 12px;font-size:var(--text-base);cursor:pointer;",
      "Clean…"
    );
    clean.setAttribute("type", "button");
    children.push(clean);

    paint(children);
  }

  async function poll() {
    if (state.stopped) return;
    try {
      const response = await ctx.fetch("summary");
      if (!response.ok) {
        // A 502 is core saying the subprocess is not listening yet: core
        // restarts it every two seconds, so this is "starting", not "broken".
        if (response.status === 502 || response.status === 503 || response.status === 504) {
          state.attempts += 1;
          if (state.attempts < 5) {
            starting();
            return;
          }
        }
        failed(String(response.status));
        return;
      }
      const summary = await response.json();
      state.attempts = 0;
      loaded(summary);
    } catch (error) {
      failed(error instanceof Error ? error.message : String(error));
    }
  }

  starting();
  void poll();
  state.timer = setInterval(poll, Math.max(1, ctx.pollSeconds || 10) * 1000);

  return () => {
    state.stopped = true;
    if (state.timer !== null) clearInterval(state.timer);
  };
}
