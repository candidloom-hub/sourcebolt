"use strict";
// End-to-end checks of bin/sourcebolt.js. SOURCEBOLT_BINARY points at node itself so the
// "binary" is portable; real platform-package resolution is covered in platform.test.js.
const test = require("node:test");
const assert = require("node:assert/strict");
const { spawnSync } = require("node:child_process");
const path = require("node:path");

const launcher = path.join(__dirname, "..", "bin", "sourcebolt.js");
const withNode = { ...process.env, SOURCEBOLT_BINARY: process.execPath };

test("forwards args and exit code from the resolved binary", () => {
  const r = spawnSync(process.execPath, [launcher, "-e", "process.exit(7)"], { env: withNode, encoding: "utf8" });
  assert.equal(r.status, 7);
});

test("passes stdout through (stdio inherit)", () => {
  const r = spawnSync(process.execPath, [launcher, "-p", "'hi'"], { env: withNode, encoding: "utf8" });
  assert.equal(r.status, 0);
  assert.match(r.stdout, /hi/);
});

test("missing platform package exits 1 with a clear message", () => {
  const env = { ...process.env };
  delete env.SOURCEBOLT_BINARY;
  const r = spawnSync(process.execPath, [launcher, "--version"], { env, encoding: "utf8" });
  assert.equal(r.status, 1);
  assert.match(r.stderr, /sourcebolt: /);
});
