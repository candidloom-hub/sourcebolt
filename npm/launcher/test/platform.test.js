"use strict";
const test = require("node:test");
const assert = require("node:assert/strict");
const p = require("../lib/platform.js");

test("maps every supported platform to a scoped package", () => {
  assert.equal(p.SUPPORTED.length, 6);
  const t = p.resolveTarget("linux", "x64");
  assert.equal(t.pkg, "@candidloom-hub/sourcebolt-linux-x64");
  assert.equal(t.file, "sourcebolt");
  assert.equal(t.spec.goarch, "amd64");
  const w = p.resolveTarget("win32", "arm64");
  assert.equal(w.pkg, "@candidloom-hub/sourcebolt-win32-arm64");
  assert.equal(w.file, "sourcebolt.exe");
  assert.equal(w.spec.goos, "windows");
  assert.equal(p.resolveTarget("darwin", "arm64").pkg, "@candidloom-hub/sourcebolt-darwin-arm64");
});

test("unsupported platform returns null and a message listing supported ones", () => {
  assert.equal(p.resolveTarget("freebsd", "x64"), null);
  assert.equal(p.resolveTarget("linux", "ia32"), null);
  const msg = p.unsupportedMessage("freebsd", "x64");
  assert.match(msg, /unsupported platform freebsd-x64/);
  for (const s of p.SUPPORTED) assert.ok(msg.includes(`${s.os}-${s.cpu}`), s.os + s.cpu);
});

test("locateBinary throws the unsupported message", () => {
  assert.throws(() => p.locateBinary("sunos", "x64", () => "x", {}), /Supported platforms/);
});

test("locateBinary resolves via the injected resolver", () => {
  const seen = [];
  const got = p.locateBinary("win32", "x64", (id) => (seen.push(id), "C:\\a b\\sourcebolt.exe"), {});
  assert.equal(got, "C:\\a b\\sourcebolt.exe");
  assert.deepEqual(seen, ["@candidloom-hub/sourcebolt-win32-x64/bin/sourcebolt.exe"]);
});

test("locateBinary reports a missing optional package", () => {
  const boom = () => {
    throw new Error("MODULE_NOT_FOUND");
  };
  assert.throws(() => p.locateBinary("linux", "arm64", boom, {}), /not installed[\s\S]*optionalDependency/);
});

test("SOURCEBOLT_BINARY overrides resolution", () => {
  assert.equal(p.locateBinary("sunos", "x64", () => "x", { SOURCEBOLT_BINARY: "/opt/sb" }), "/opt/sb");
});
