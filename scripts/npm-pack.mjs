#!/usr/bin/env node
// Builds the npm packages (one per platform + the launcher) from GoReleaser's dist/ and
// runs `npm pack --dry-run` on each. It never publishes; the release workflow does that
// separately and only when the publish_npm input is true.
//
//   node scripts/npm-pack.mjs --version 0.1.0 [--dist dist] [--out dist/npm]
//                             [--platforms linux-x64,win32-x64] [--allow-missing] [--no-pack]
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const require = createRequire(import.meta.url);
const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, "..");
const { SUPPORTED, packageName, binaryFile } = require(path.join(repo, "npm/launcher/lib/platform.js"));

// Finds the GoReleaser output dir for a platform: dist/sourcebolt_<goos>_<goarch>[_v1|_v8.0].
// The browser build (sourcebolt-browser_*) is never matched.
export function findBinary(dist, spec, platform = spec.os) {
  if (!fs.existsSync(dist)) return null;
  const prefix = `sourcebolt_${spec.goos}_${spec.goarch}`;
  const dir = fs
    .readdirSync(dist, { withFileTypes: true })
    .find((d) => d.isDirectory() && (d.name === prefix || d.name.startsWith(prefix + "_")));
  if (!dir) return null;
  const file = path.join(dist, dir.name, binaryFile(platform));
  return fs.existsSync(file) ? file : null;
}

export function renderTemplate(tpl, vars) {
  return tpl.replace(/\{\{(\w+)\}\}/g, (_, k) => {
    if (!(k in vars)) throw new Error(`unknown template variable ${k}`);
    return vars[k];
  });
}

export function parseArgs(argv) {
  const o = { dist: "dist", out: null, platforms: null, allowMissing: false, pack: true, version: null };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--version") o.version = argv[++i];
    else if (a === "--dist") o.dist = argv[++i];
    else if (a === "--out") o.out = argv[++i];
    else if (a === "--platforms") o.platforms = argv[++i].split(",");
    else if (a === "--allow-missing") o.allowMissing = true;
    else if (a === "--no-pack") o.pack = false;
    else throw new Error(`unknown argument ${a}`);
  }
  if (!o.version) throw new Error("--version is required (semver, no leading v)");
  o.version = o.version.replace(/^v/, "");
  if (!/^\d+\.\d+\.\d+([-+].+)?$/.test(o.version)) throw new Error(`bad version ${o.version}`);
  o.out = o.out || path.join(o.dist, "npm");
  return o;
}

function copyIfExists(src, dst) {
  if (fs.existsSync(src)) fs.copyFileSync(src, dst);
}

function npmPackDryRun(dir) {
  const r = spawnSync("npm", ["pack", "--dry-run", "--json"], { cwd: dir, encoding: "utf8", shell: process.platform === "win32" });
  if (r.status !== 0) throw new Error(`npm pack --dry-run failed in ${dir}:\n${r.stderr}`);
  const parsed = JSON.parse(r.stdout);
  // npm <=10 prints an array; newer npm prints an object keyed by package name.
  const info = Array.isArray(parsed) ? parsed[0] : Object.values(parsed)[0];
  console.log(`  ${info.name}@${info.version}: ${info.files.length} files, ${info.size} bytes packed`);
}

export function build(opts) {
  const tpl = fs.readFileSync(path.join(repo, "npm/platform.template.json"), "utf8");
  const wanted = opts.platforms
    ? SUPPORTED.filter((s) => opts.platforms.includes(`${s.os}-${s.cpu}`))
    : SUPPORTED;
  if (opts.platforms && wanted.length !== opts.platforms.length) {
    throw new Error(`unknown platform in --platforms ${opts.platforms.join(",")}`);
  }
  fs.rmSync(opts.out, { recursive: true, force: true });
  fs.mkdirSync(opts.out, { recursive: true });

  const built = [];
  for (const spec of wanted) {
    const bin = findBinary(opts.dist, spec);
    if (!bin) {
      const msg = `no binary for ${spec.os}-${spec.cpu} (${spec.goos}/${spec.goarch}) in ${opts.dist}`;
      if (opts.allowMissing) {
        console.warn(`warning: ${msg}; skipping`);
        continue;
      }
      throw new Error(msg);
    }
    const dir = path.join(opts.out, `sourcebolt-${spec.os}-${spec.cpu}`);
    fs.mkdirSync(path.join(dir, "bin"), { recursive: true });
    const dst = path.join(dir, "bin", binaryFile(spec.os));
    fs.copyFileSync(bin, dst);
    fs.chmodSync(dst, 0o755);
    fs.writeFileSync(path.join(dir, "package.json"), renderTemplate(tpl, { os: spec.os, cpu: spec.cpu, version: opts.version }));
    copyIfExists(path.join(repo, "LICENSE"), path.join(dir, "LICENSE"));
    copyIfExists(path.join(repo, "NOTICE"), path.join(dir, "NOTICE"));
    built.push({ name: packageName(spec.os, spec.cpu), dir });
  }

  // Launcher: copy bin/ and lib/, pin the version and every optionalDependency to it.
  const ldir = path.join(opts.out, "sourcebolt");
  fs.mkdirSync(ldir, { recursive: true });
  fs.cpSync(path.join(repo, "npm/launcher/bin"), path.join(ldir, "bin"), { recursive: true });
  fs.cpSync(path.join(repo, "npm/launcher/lib"), path.join(ldir, "lib"), { recursive: true });
  const pkg = JSON.parse(fs.readFileSync(path.join(repo, "npm/launcher/package.json"), "utf8"));
  pkg.version = opts.version;
  delete pkg.scripts;
  for (const k of Object.keys(pkg.optionalDependencies)) pkg.optionalDependencies[k] = opts.version;
  fs.writeFileSync(path.join(ldir, "package.json"), JSON.stringify(pkg, null, 2) + "\n");
  copyIfExists(path.join(repo, "LICENSE"), path.join(ldir, "LICENSE"));
  copyIfExists(path.join(repo, "NOTICE"), path.join(ldir, "NOTICE"));
  built.push({ name: "sourcebolt", dir: ldir });

  if (opts.pack) {
    console.log("npm pack --dry-run:");
    for (const b of built) npmPackDryRun(b.dir);
  }
  return built;
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    const built = build(parseArgs(process.argv.slice(2)));
    console.log(`prepared ${built.length} package(s)`);
  } catch (e) {
    console.error(`npm-pack: ${e.message}`);
    process.exit(1);
  }
}
