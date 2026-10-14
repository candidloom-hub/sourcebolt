import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { findBinary, renderTemplate, parseArgs, build } from "./npm-pack.mjs";

function fakeDist() {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), "sb dist-"));
  for (const [dir, file] of [
    ["sourcebolt_linux_amd64_v1", "sourcebolt"],
    ["sourcebolt_windows_amd64_v1", "sourcebolt.exe"],
    ["sourcebolt-browser_linux_amd64_v1", "sourcebolt"],
  ]) {
    fs.mkdirSync(path.join(d, dir));
    fs.writeFileSync(path.join(d, dir, file), "bin");
  }
  return d;
}

test("findBinary matches goreleaser dirs and ignores the browser build", () => {
  const d = fakeDist();
  const lin = findBinary(d, { os: "linux", goos: "linux", goarch: "amd64" });
  assert.ok(lin.includes("sourcebolt_linux_amd64_v1"));
  const win = findBinary(d, { os: "win32", goos: "windows", goarch: "amd64" });
  assert.ok(win.endsWith("sourcebolt.exe"));
  assert.equal(findBinary(d, { os: "linux", goos: "linux", goarch: "arm64" }), null);
});

test("renderTemplate substitutes and rejects unknown variables", () => {
  assert.equal(renderTemplate("{{a}}-{{b}}", { a: "x", b: "y" }), "x-y");
  assert.throws(() => renderTemplate("{{zzz}}", {}), /unknown template variable/);
});

test("parseArgs requires a valid version and strips v", () => {
  assert.throws(() => parseArgs([]), /--version is required/);
  assert.throws(() => parseArgs(["--version", "abc"]), /bad version/);
  assert.equal(parseArgs(["--version", "v1.2.3"]).version, "1.2.3");
});

test("build writes platform packages and a pinned launcher (no pack)", () => {
  const d = fakeDist();
  const out = path.join(d, "npm");
  const built = build({ dist: d, out, platforms: ["linux-x64", "win32-x64"], allowMissing: false, pack: false, version: "1.2.3" });
  assert.equal(built.length, 3);
  const p = JSON.parse(fs.readFileSync(path.join(out, "sourcebolt-win32-x64", "package.json"), "utf8"));
  assert.equal(p.name, "@candidloom-hub/sourcebolt-win32-x64");
  assert.deepEqual(p.os, ["win32"]);
  assert.deepEqual(p.cpu, ["x64"]);
  assert.ok(fs.existsSync(path.join(out, "sourcebolt-win32-x64", "bin", "sourcebolt.exe")));
  const l = JSON.parse(fs.readFileSync(path.join(out, "sourcebolt", "package.json"), "utf8"));
  assert.equal(l.version, "1.2.3");
  assert.ok(Object.values(l.optionalDependencies).every((v) => v === "1.2.3"));
});

test("build fails on a missing binary unless allowMissing", () => {
  const d = fakeDist();
  const base = { dist: d, out: path.join(d, "o"), platforms: ["darwin-arm64"], pack: false, version: "1.0.0" };
  assert.throws(() => build({ ...base, allowMissing: false }), /no binary for darwin-arm64/);
  assert.equal(build({ ...base, allowMissing: true }).length, 1);
});
