import { createServer } from "node:http";
import os from "node:os";
import fs from "node:fs";
import { exec } from "node:child_process";
import { promisify } from "node:util";

const execPromise = promisify(exec);

const port = Number(process.env.STAR_PANEL_PORT ?? "{port}");
const plugin = process.env.STAR_PANEL_PLUGIN ?? "system-stats";

// ------------------- CPU -------------------
function sampleCpu(ms = 1000) {
    return new Promise((resolve) => {
        const start = os.cpus();
        setTimeout(() => {
            const end = os.cpus();
            let idleDiff = 0;
            let totalDiff = 0;
            for (let i = 0; i < start.length; i++) {
                const t1 = start[i].times;
                const t2 = end[i].times;
                const idle = t2.idle - t1.idle;
                const total =
                    t2.user + t2.nice + t2.sys + t2.idle + t2.irq -
                    (t1.user + t1.nice + t1.sys + t1.idle + t1.irq);
                idleDiff += idle;
                totalDiff += total;
            }
            resolve(totalDiff > 0 ? (1 - idleDiff / totalDiff) * 100 : 0);
        }, ms);
    });
}

// ------------------- Disco -------------------
async function readDisk(mount = process.platform === "win32" ? "C:" : "/") {
    if (typeof fs.statfs === "function") {
        try {
            const s = await fs.promises.statfs(mount);
            const total = s.blocks * s.bsize;
            const free = s.bfree * s.bsize;
            return { total, used: total - free };
        } catch { }
    }
    if (process.platform === "win32") {
        const { stdout } = await execPromise(
            `wmic logicaldisk where "DeviceID='${mount}'" get Size,FreeSpace /format:csv`
        );
        const line = stdout.trim().split(/\r?\n/).filter((l) => l.trim())[1];
        const [, freeStr, totalStr] = line.split(",");
        const total = Number(totalStr);
        const free = Number(freeStr);
        return { total, used: total - free };
    }
    const { stdout } = await execPromise(`df -kP "${mount}"`);
    const parts = stdout.trim().split("\n")[1].split(/\s+/);
    const total = Number(parts[1]) * 1024;
    const used = Number(parts[2]) * 1024;
    return { total, used };
}

// ------------------- Stats -------------------
async function buildStats() {
    const cpuPercent = await sampleCpu();

    const memTotalBytes = os.totalmem();
    const memUsedBytes = memTotalBytes - os.freemem();

    const disk = await readDisk();

    const source =
        process.platform === "linux" ? "procfs" :
            process.platform === "darwin" ? "sysctl" :
                process.platform === "win32" ? "win32" : "unknown";

    return {
        cpuPercent: Number(cpuPercent.toFixed(1)),
        memUsedBytes,
        memTotalBytes,
        diskUsedBytes: disk.used,
        diskTotalBytes: disk.total,
        source,
    };
}

// ------------------- Server -------------------
const server = createServer(async (req, res) => {
    const json = (status, body) => {
        res.writeHead(status, { "Content-Type": "application/json" });
        res.end(JSON.stringify(body));
    };

    try {
        if (req.method === "GET" && req.url === "/ping") {
            json(200, { ok: true, plugin, time: Date.now() });
            return;
        }

        if (req.method === "GET" && req.url === "/stats") {
            json(200, await buildStats());
            return;
        }

        json(404, { error: "no such route" });
    } catch (err) {
        json(500, { error: err.message });
    }
});

server.listen(port, "127.0.0.1", () => {
    console.log(`${plugin} backend on 127.0.0.1:${port}`);
});