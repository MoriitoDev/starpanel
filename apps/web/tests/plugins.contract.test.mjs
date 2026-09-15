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

// disk-usage is the Plugin that brings its own binary: the widget reads a
// report the backend compiled from the operating system's own calls.
const diskReport = {
  source: "win32",
  volumes: [
    {
      name: "C:",
      label: "Windows",
      fs: "NTFS",
      mount: "C:\\",
      totalBytes: 1024 ** 3,
      freeBytes: 512 * 1024 ** 2,
      usedBytes: 512 * 1024 ** 2,
      usedPercent: 50
    },
    {
      name: "D:",
      label: "Data",
      fs: "NTFS",
      mount: "D:\\",
      totalBytes: 2 * 1024 ** 3,
      freeBytes: 1.5 * 1024 ** 3,
      usedBytes: 0.5 * 1024 ** 3,
      usedPercent: 25
    }
  ]
};

// A DOM stub wide enough for the disk-usage widget: nested elements, styles,
// attributes and text.
function diskStubElement() {
  return {
    children: [],
    style: { cssText: "" },
    textContent: "",
    attributes: {},
    append(...els) {
      this.children.push(...els);
    },
    replaceChildren(...els) {
      this.children = els;
    },
    setAttribute(name, value) {
      this.attributes[name] = value;
    }
  };
}

/** Every piece of text in the tree, so a test can read what a person reads. */
function textsDeep(node) {
  const own = typeof node.textContent === "string" && node.textContent !== "" ? [node.textContent] : [];
  return [...own, ...(node.children ?? []).flatMap(textsDeep)];
}

/** The styles in the tree, so a test can see which Tokens were painted. */
function stylesDeep(node) {
  const own = typeof node.style?.cssText === "string" && node.style.cssText !== "" ? [node.style.cssText] : [];
  return [...own, ...(node.children ?? []).flatMap(stylesDeep)];
}

/** Render the widget against a fake backend answer and wait for it to land. */
async function renderDiskUsage(ctx = {}) {
  const mod = await import(pathToFileURL(`${pluginsDir}disk-usage/widget.js`).href);
  const el = diskStubElement();
  const cleanup = mod.default(el, {
    config: {},
    pollSeconds: 10,
    fetch: async () => ({ ok: true, status: 200, json: async () => diskReport }),
    ...ctx
  });
  for (let attempt = 0; attempt < 50; attempt += 1) {
    if (textsDeep(el).length > 1) break;
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
  return { el, cleanup };
}

test("disk-usage manifest declares a widget and a backend binary", () => {
  const m = readManifest("disk-usage");
  assert.equal(m.name, "disk-usage");
  assert.ok(m.version, "manifest declares a version");
  assert.equal(m.widgets.length, 1, "the Plugin provides one widget");
  assert.ok(
    existsSync(`${pluginsDir}disk-usage/${m.widgets[0].module}`),
    `module ${m.widgets[0].module} exists`
  );
  assert.ok(Array.isArray(m.backend?.command) && m.backend.command.length > 0, "backend command declared");
  const binary = m.backend.command[m.backend.command.length - 1];
  assert.ok(existsSync(`${pluginsDir}disk-usage/${binary}`), `backend binary ${binary} ships with the Plugin`);
  assert.ok(
    m.requires?.some((r) => r.command === binary),
    "the Manifest says out loud what it needs, so a machine without it is told"
  );
});

test("disk-usage widget lists every volume with its free space", async (t) => {
  globalThis.document = { createElement: () => diskStubElement() };
  const { el, cleanup } = await renderDiskUsage();
  t.after(cleanup);

  const texts = textsDeep(el);
  assert.ok(texts.includes("C: · Windows · NTFS"), "a volume is named by letter, label and filesystem");
  assert.ok(texts.includes("50% · 512 MB free"), "and carries how full it is and what is left");
  assert.ok(texts.includes("D: · Data · NTFS"));
  assert.ok(texts.includes("25% · 1.5 GB free"));
  assert.ok(texts.includes("2 volumes · source: win32"), "the footer says where the numbers came from");
  assert.equal(typeof cleanup, "function", "render returns a cleanup function");
});

test("disk-usage widget says in words when a volume is nearly full", async (t) => {
  globalThis.document = { createElement: () => diskStubElement() };
  const nearlyFull = {
    source: "procfs",
    volumes: [
      {
        name: "/",
        fs: "ext4",
        mount: "/",
        totalBytes: 1000,
        freeBytes: 100,
        usedBytes: 900,
        usedPercent: 90
      }
    ]
  };
  const { el, cleanup } = await renderDiskUsage({
    fetch: async () => ({ ok: true, status: 200, json: async () => nearlyFull })
  });
  t.after(cleanup);

  const texts = textsDeep(el);
  assert.ok(
    texts.some((text) => text.startsWith("/ · ext4")),
    "the volume is named"
  );
  assert.ok(texts.includes("nearly full"), "the state is written down, not only coloured");
  assert.ok(texts.includes("90% · 100 B free"));
  assert.ok(
    stylesDeep(el).some((css) => css.includes("var(--danger)")),
    "and the failing volume is painted with the state Token"
  );
});

test("disk-usage widget takes its warning line from the config", async (t) => {
  globalThis.document = { createElement: () => diskStubElement() };
  const halfFull = {
    source: "win32",
    volumes: [
      {
        name: "C:",
        fs: "NTFS",
        mount: "C:\\",
        totalBytes: 1000,
        freeBytes: 400,
        usedBytes: 600,
        usedPercent: 60
      }
    ]
  };
  const { el, cleanup } = await renderDiskUsage({
    config: { warnPercent: 50 },
    fetch: async () => ({ ok: true, status: 200, json: async () => halfFull })
  });
  t.after(cleanup);

  assert.match(textsDeep(el).join(" | "), /nearly full/, "an owner may warn earlier than the default 90%");
});

test("disk-usage widget reports a failed backend instead of breaking the panel", async (t) => {
  globalThis.document = { createElement: () => diskStubElement() };
  const { el, cleanup } = await renderDiskUsage({
    fetch: async () => ({ ok: false, status: 500, json: async () => ({ error: "read mounts: permission denied" }) })
  });
  t.after(cleanup);

  const text = textsDeep(el).join(" | ");
  assert.match(text, /Could not read the disks/, "the widget says what went wrong");
  assert.match(text, /500/, "and names the answer that failed");
  assert.ok(stylesDeep(el).some((css) => css.includes("var(--danger)")));
});

test("disk-usage widget shows an empty machine as empty", async (t) => {
  globalThis.document = { createElement: () => diskStubElement() };
  const { el, cleanup } = await renderDiskUsage({
    fetch: async () => ({ ok: true, status: 200, json: async () => ({ source: "statfs", volumes: [] }) })
  });
  t.after(cleanup);

  assert.match(textsDeep(el).join(" | "), /No volumes found/);
});
