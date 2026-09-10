// hello-widget: the reference frontend-only widget.
//
// A widget module is plain ESM with one default export:
//   render(el, ctx) -> optional cleanup function
// where `el` is a container element to render into and `ctx` is
//   { config, theme, pollSeconds, fetch }
// (fetch targets this plugin's /proxy/ path; it is only useful for
// plugins with a backend). Styling follows docs/PLUGINS.md: the design
// Tokens as CSS variables, never a literal colour, so the widget follows
// light, dark and auto with the rest of the panel.
export default function render(el, ctx) {
  const config = ctx.config ?? {};
  const message =
    typeof config.message === "string" && config.message !== ""
      ? config.message
      : "Hello, Star Panel!";

  const line = document.createElement("p");
  line.textContent = message;
  line.style.cssText =
    "margin:0 0 4px;color:var(--ink);font-size:var(--text-base);font-weight:600;";

  const meta = document.createElement("p");
  meta.style.cssText =
    "margin:0;color:var(--mute);font-size:var(--text-meta);line-height:var(--text-meta--line-height);";

  el.replaceChildren(line, meta);

  let ticks = 0;
  const tick = () => {
    ticks += 1;
    meta.textContent = `poll every ${ctx.pollSeconds}s · tick ${ticks}`;
  };
  tick();
  const timer = setInterval(tick, Math.max(1, Number(ctx.pollSeconds) || 10) * 1000);
  return () => clearInterval(timer);
}
