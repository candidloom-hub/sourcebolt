#!/usr/bin/env node
"use strict";
const { spawn } = require("node:child_process");
const { locateBinary } = require("../lib/platform.js");

let bin;
try {
  bin = locateBinary(process.platform, process.arch, require.resolve, process.env);
} catch (e) {
  process.stderr.write(`${e.message}\n`);
  process.exit(1);
}

const child = spawn(bin, process.argv.slice(2), { stdio: "inherit", windowsHide: true });

// Forward termination signals so the server shuts down cleanly.
for (const sig of ["SIGINT", "SIGTERM", "SIGHUP"]) {
  process.on(sig, () => {
    if (!child.killed) child.kill(sig);
  });
}

child.on("error", (err) => {
  process.stderr.write(`sourcebolt: failed to start ${bin}: ${err.message}\n`);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    // Re-raise so our parent sees the same signal.
    process.removeAllListeners(signal);
    try {
      process.kill(process.pid, signal);
    } catch {
      process.exit(1);
    }
    return;
  }
  process.exit(code === null ? 1 : code);
});
