// Plugin contract tests (seam 2): the shipped plugin folders honor the
// Manifest schema and the widget module contract. Run: pnpm test
import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";

const pluginsDir = fileURLToPath(new URL("../../api/plugins/", import.meta.url));

function readManifest(plugin) {
  return JSON.parse(readFileSync(`${pluginsDir}${plugin}/manifest.json`, "utf8"));
}

test("hello-widget manifest is valid and frontend-only", () => {
  const m = readManifest("hello-widget");
  assert.equal(m.name, "hello-widget");
  assert.ok(m.version, "manifest declares a version");
  assert.ok(m.widgets.length >= 1, "manifest provides at least one widget");
  for (const w of m.widgets) {
    assert.ok(w.id, "widget has an id");
    assert.ok(existsSync(`${pluginsDir}hello-widget/${w.module}`), `module ${w.module} exists`);
  }
  assert.equal(m.backend, undefined, "hello-widget declares no backend");
});

test("widget module renders with stub element and context", async () => {
  const m = readManifest("hello-widget");

  // Minimal DOM stub covering what the widget module uses.
  const stubElement = () => ({
    children: [],
    style: { cssText: "" },
    textContent: "",
    append(...els) {
      this.children.push(...els);
    },
    replaceChildren(...els) {
      this.children = els;
    }
  });
  globalThis.document = { createElement: () => stubElement() };

  const mod = await import(pathToFileURL(`${pluginsDir}hello-widget/${m.widgets[0].module}`).href);
  assert.equal(typeof mod.default, "function", "widget exports a default render function");

  const el = stubElement();
  const result = mod.default(el, {
    config: { message: "Hi from the contract test" },
    pollSeconds: 10,
    fetch: async () => {
      throw new Error("no backend");
    }
  });

  assert.equal(typeof result, "function", "render returns a cleanup function");
  assert.ok(el.children.length >= 2, "widget rendered content into the element");
  assert.match(el.children[0].textContent, /Hi from the contract test/);
  assert.match(el.children[1].textContent, /poll every 10s/);
  result();
});

test("widget config falls back without a message", async () => {
  const stubElement = () => ({
    children: [],
    style: { cssText: "" },
    textContent: "",
    append(...els) {
      this.children.push(...els);
    },
    replaceChildren(...els) {
      this.children = els;
    }
  });
  globalThis.document = { createElement: () => stubElement() };

  const mod = await import(pathToFileURL(`${pluginsDir}hello-widget/widget.js`).href);
  const el = stubElement();
  const cleanup = mod.default(el, {
    config: {},
    pollSeconds: 5,
    fetch: async () => {
      throw new Error("no backend");
    }
  });
  assert.match(el.children[0].textContent, /Hello, Star Panel!/);
  cleanup();
});

test("echo plugin declares a backend command that exists on disk", () => {
  const m = readManifest("echo");
  assert.equal(m.name, "echo");
  assert.ok(Array.isArray(m.backend?.command) && m.backend.command.length > 0, "backend command declared");
  const script = m.backend.command[m.backend.command.length - 1];
  assert.ok(existsSync(`${pluginsDir}echo/${script}`), `backend script ${script} exists`);
});
