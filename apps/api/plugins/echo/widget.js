// echo widget: consumes this plugin's backend through ctx.fetch, which
// targets the core proxy path /api/v1/plugins/echo/proxy/.
// Styling follows docs/PLUGINS.md: design Tokens as CSS variables only.
export default function render(el, ctx) {
  const status = document.createElement("p");
  status.style.cssText =
    "margin:0;font-size:var(--text-base);font-weight:600;color:var(--mute);";
  const meta = document.createElement("p");
  meta.style.cssText =
    "margin:2px 0 0;color:var(--mute);font-size:var(--text-meta);line-height:var(--text-meta--line-height);";
  el.replaceChildren(status, meta);

  const pollMs = Math.max(1, Number(ctx.pollSeconds) || 10) * 1000;

  async function refresh() {
    try {
      const res = await ctx.fetch("ping");
      if (!res.ok) throw new Error(`backend ${res.status}`);
      const data = await res.json();
      status.style.color = "var(--success)";
      status.textContent = "backend alive";
      meta.textContent = `last ping ${new Date(data.time).toLocaleTimeString()} · poll every ${ctx.pollSeconds}s`;
    } catch (err) {
      // Crashed or stopped backends surface here as a widget-friendly
      // message; the dashboard itself keeps working.
      status.style.color = "var(--danger)";
      status.textContent = "backend unavailable";
      meta.textContent = String(err);
    }
  }

  refresh();
  const timer = setInterval(refresh, pollMs);
  return () => clearInterval(timer);
}
