// system-stats: built-in plugin widget. The backend lives inside core and
// answers on this plugin's proxy path at /stats; the widget just polls it
// like any other backend plugin.
// Styling follows docs/PLUGINS.md: design Tokens as CSS variables only.
const FIELDS = [
  { key: "cpu", label: "CPU" },
  { key: "mem", label: "Memory" },
  { key: "disk", label: "Disk" }
];

function formatBytes(n) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = Number(n) || 0;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(value >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`;
}

export default function render(el, ctx) {
  const rows = FIELDS.map(({ key, label }) => {
    const row = document.createElement("div");
    row.style.cssText = "margin:0 0 10px;";
    const head = document.createElement("div");
    head.style.cssText =
      "display:flex;justify-content:space-between;gap:12px;color:var(--mute);font-size:var(--text-meta);line-height:var(--text-meta--line-height);";
    const name = document.createElement("span");
    name.textContent = label;
    const value = document.createElement("span");
    value.style.cssText = "color:var(--ink);font-variant-numeric:tabular-nums;";
    head.append(name, value);
    const bar = document.createElement("div");
    bar.style.cssText =
      "margin-top:6px;height:4px;border-radius:var(--radius-pill);overflow:hidden;background:var(--surface-soft);";
    const fill = document.createElement("div");
    fill.style.cssText =
      "height:100%;width:0;border-radius:var(--radius-pill);background:var(--ink);transition:width var(--default-transition-duration) var(--ease-quiet);";
    bar.append(fill);
    row.append(head, bar);
    el.append(row);
    return { key, value, fill };
  });

  const note = document.createElement("p");
  note.style.cssText =
    "margin:2px 0 0;color:var(--mute);font-size:var(--text-meta);line-height:var(--text-meta--line-height);";
  el.append(note);

  function apply(data) {
    const pct = (used, total) =>
      total > 0 ? Math.min(100, Math.round((used / total) * 100)) : 0;
    for (const row of rows) {
      if (row.key === "cpu") {
        const cpu = Math.round(Number(data.cpuPercent) || 0);
        row.value.textContent = `${cpu}%`;
        row.fill.style.width = `${cpu}%`;
      } else if (row.key === "mem") {
        row.value.textContent = `${formatBytes(data.memUsedBytes)} / ${formatBytes(data.memTotalBytes)}`;
        row.fill.style.width = `${pct(data.memUsedBytes, data.memTotalBytes)}%`;
      } else {
        row.value.textContent = `${formatBytes(data.diskUsedBytes)} / ${formatBytes(data.diskTotalBytes)}`;
        row.fill.style.width = `${pct(data.diskUsedBytes, data.diskTotalBytes)}%`;
      }
    }
    note.textContent = data.source === "stub"
      ? "dev stub — real stats on Linux only"
      : `source: ${data.source}`;
  }

  async function refresh() {
    try {
      const res = await ctx.fetch("stats");
      if (!res.ok) throw new Error(`backend ${res.status}`);
      apply(await res.json());
    } catch (err) {
      note.textContent = `stats unavailable: ${err}`;
    }
  }

  refresh();
  const timer = setInterval(refresh, Math.max(1, Number(ctx.pollSeconds) || 10) * 1000);
  return () => clearInterval(timer);
}
