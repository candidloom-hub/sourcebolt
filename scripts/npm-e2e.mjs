#!/usr/bin/env node
// Local install smoke test (no registry, no publishing): packs the launcher and the
// current-platform package from dist/npm into tarballs, installs them into a temp project
// whose path contains spaces, and runs `sourcebolt --version` through the launcher.
//   node scripts/npm-e2e.mjs [--out dist/npm]
import { execSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const out = path.resolve(process.argv[2] === "--out" ? process.argv[3] : "dist/npm");
const plat = `${process.platform}-${process.arch}`;
const platDir = path.join(out, `sourcebolt-${plat}`);
const launcherDir = path.join(out, "sourcebolt");
for (const d of [platDir, launcherDir]) {
  if (!fs.existsSync(d)) {
    console.error(`missing ${d}; run scripts/npm-pack.mjs first (platform ${plat})`);
    process.exit(1);
  }
}
const root = fs.mkdtempSync(path.join(os.tmpdir(), "sb e2e npm "));
const tgz = path.join(root, "tgz");
const proj = path.join(root, "proj");
fs.mkdirSync(tgz);
fs.mkdirSync(proj);
const run = (cmd, cwd) => execSync(cmd, { cwd, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });

run(`npm pack --pack-destination "${tgz}"`, platDir);
run(`npm pack --pack-destination "${tgz}"`, launcherDir);
const files = fs.readdirSync(tgz).map((f) => `"${path.join(tgz, f)}"`).join(" ");
run("npm init -y", proj);
run(`npm install --no-audit --no-fund ${files}`, proj);
const version = run("npx --no-install sourcebolt --version", proj).trim();
console.log(`launcher output: ${version}`);
let code = 0;
try {
  run("npx --no-install sourcebolt definitely-not-a-command", proj);
} catch (e) {
  code = e.status;
}
console.log(`unknown command exit code forwarded: ${code}`);
if (!/^sourcebolt /.test(version) || code === 0) process.exit(1);
fs.rmSync(root, { recursive: true, force: true });
console.log("ok");
