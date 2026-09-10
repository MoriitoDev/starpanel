// echo: a backend plugin in plain Node — any language that speaks HTTP on
// localhost works. Core allocates the port and passes it as STAR_PANEL_PORT
// (and by replacing {port} in the Manifest command arguments).
import { createServer } from "node:http";

const port = Number(process.env.STAR_PANEL_PORT ?? "{port}");
const plugin = process.env.STAR_PANEL_PLUGIN ?? "echo";

const server = createServer((req, res) => {
  const json = (status, body) => {
    res.writeHead(status, { "Content-Type": "application/json" });
    res.end(JSON.stringify(body));
  };
  if (req.method === "GET" && req.url === "/ping") {
    json(200, { ok: true, plugin, time: Date.now() });
    return;
  }
  json(404, { error: "no such route" });
});

server.listen(port, "127.0.0.1", () => {
  console.log(`echo backend on 127.0.0.1:${port}`);
});
