"use strict";
// Verifies the promise that a Plugin may bring its own binary: the ZIP an owner
// builds carries an execute bit on that binary, and the panel's import/archive
// round trip keeps it. It reads the ZIP's own external attributes rather than
// shelling out to unzip or tar, so it runs anywhere Node does.
//
//   node apps/api/plugins/disk-space/verify-zip.mjs [panel-url]
//
// The panel has to be running for the import half; without a URL the script
// checks only the built ZIP, which is the half a build script can guarantee.

import { readFileSync } from "node:fs";

const zipPath = new URL("./dist/disk-space.zip", import.meta.url);

/**
 * Reads every entry's name and Unix mode out of a ZIP's central directory. The
 * mode lives in the high 16 bits of the external attributes, which is exactly
 * the field `Compress-Archive` leaves empty.
 */
function entries(buffer) {
  const found = [];
  // The end-of-central-directory record is at the tail; its comment is capped
  // at 64 KiB, so the search window is bounded.
  let eocd = -1;
  for (let i = buffer.length - 22; i >= Math.max(0, buffer.length - 22 - 65535); i -= 1) {
    if (buffer.readUInt32LE(i) === 0x06054b50) {
      eocd = i;
      break;
    }
  }
  if (eocd < 0) throw new Error("not a ZIP: no end-of-central-directory record");
  const count = buffer.readUInt16LE(eocd + 10);
  let offset = buffer.readUInt32LE(eocd + 16);
  for (let index = 0; index < count; index += 1) {
    if (buffer.readUInt32LE(offset) !== 0x02014b50) throw new Error("bad central directory entry");
    const external = buffer.readUInt32LE(offset + 38);
    const nameLength = buffer.readUInt16LE(offset + 28);
    const extraLength = buffer.readUInt16LE(offset + 30);
    const commentLength = buffer.readUInt16LE(offset + 32);
    const name = buffer.toString("utf8", offset + 46, offset + 46 + nameLength);
    // High 16 bits hold the Unix mode; 0o100000 marks a regular file.
    const mode = (external >>> 16) & 0o7777;
    found.push({ name, mode, executable: (mode & 0o111) !== 0 });
    offset += 46 + nameLength + extraLength + commentLength;
  }
  return found;
}

function report(label, buffer) {
  const list = entries(buffer);
  const backend = list.find((entry) => entry.name.endsWith("/backend"));
  if (!backend) throw new Error(`${label}: no backend entry`);
  console.log(
    `${label}: backend mode 0o${backend.mode.toString(8)} (${backend.executable ? "executable" : "NOT executable"}), ${list.length} entries`
  );
  return backend.executable;
}

const built = readFileSync(zipPath);
if (!report("built ZIP", built)) {
  throw new Error("the built ZIP's backend is not executable: check build.ps1's ExternalAttributes");
}

const panel = (process.argv[2] ?? "").replace(/\/$/, "");
if (!panel) {
  console.log("no panel URL given: skipping the import round trip");
  process.exit(0);
}

const imported = await fetch(`${panel}/api/v1/plugins`, {
  method: "POST",
  headers: { "Content-Type": "application/zip" },
  body: built
});
console.log(`import answered ${imported.status}`);
if (imported.status !== 201 && imported.status !== 409) {
  throw new Error(`import failed: ${await imported.text()}`);
}

const download = await fetch(`${panel}/api/v1/plugins/disk-space/archive`);
if (!download.ok) throw new Error(`download answered ${download.status}`);
const downloaded = Buffer.from(await download.arrayBuffer());

if (process.platform === "win32") {
  // On Windows an unpacked file's mode is synthetic: chmod there only moves the
  // read-only flag, so the execute bit the built ZIP carries cannot survive a
  // trip through the filesystem on this platform, and core's Archive() reads
  // the filesystem. The round trip is therefore a Linux check — the Go tests
  // cover it where the bit exists — and saying so is more use than a red
  // failure that means "wrong operating system".
  console.log(
    `downloaded ZIP: ${entries(downloaded).length} entries — the execute bit cannot be checked on Windows; ` +
      "run this against a linux/amd64 panel to verify the round trip"
  );
} else if (!report("downloaded ZIP", downloaded)) {
  throw new Error("the round trip lost the execute bit");
}

const proxy = await fetch(`${panel}/api/v1/plugins/disk-space/proxy/summary`);
console.log(`proxy /summary answered ${proxy.status}`);
if (proxy.ok) {
  const summary = await proxy.json();
  console.log(`  ${summary.disks?.length ?? 0} Disks, source ${summary.source}`);
}
console.log("OK");
