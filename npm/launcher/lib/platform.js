"use strict";
// Platform mapping for the sourcebolt npm launcher. Pure functions so they can be unit tested.

const SCOPE = "@candidloom-hub";
const NAME = "sourcebolt";

// Node process.platform / process.arch pairs we ship binaries for.
const SUPPORTED = [
  { os: "linux", cpu: "x64", goos: "linux", goarch: "amd64" },
  { os: "linux", cpu: "arm64", goos: "linux", goarch: "arm64" },
  { os: "darwin", cpu: "x64", goos: "darwin", goarch: "amd64" },
  { os: "darwin", cpu: "arm64", goos: "darwin", goarch: "arm64" },
  { os: "win32", cpu: "x64", goos: "windows", goarch: "amd64" },
  { os: "win32", cpu: "arm64", goos: "windows", goarch: "arm64" },
];

function packageName(os, cpu) {
  return `${SCOPE}/${NAME}-${os}-${cpu}`;
}

function find(platform, arch) {
  return SUPPORTED.find((p) => p.os === platform && p.cpu === arch) || null;
}

function binaryFile(platform) {
  return platform === "win32" ? `${NAME}.exe` : NAME;
}

// Returns {pkg, file, spec} or null when the platform is unsupported.
function resolveTarget(platform, arch) {
  const spec = find(platform, arch);
  if (!spec) return null;
  return { pkg: packageName(spec.os, spec.cpu), file: binaryFile(platform), spec };
}

function unsupportedMessage(platform, arch) {
  const list = SUPPORTED.map((p) => `  - ${p.os}-${p.cpu}`).join("\n");
  return (
    `sourcebolt: unsupported platform ${platform}-${arch}.\n` +
    `Supported platforms:\n${list}\n` +
    `Download a release binary or build from source: https://github.com/candidloom-hub/sourcebolt`
  );
}

function missingPackageMessage(pkg, platform, arch) {
  return (
    `sourcebolt: the platform package ${pkg} for ${platform}-${arch} is not installed.\n` +
    `It is an optionalDependency; reinstall without --no-optional / --omit=optional, ` +
    `or install it directly: npm install ${pkg}`
  );
}

// resolveFn is require.resolve (injectable for tests). Returns an absolute path.
// Throws an Error whose message is user-ready.
function locateBinary(platform, arch, resolveFn, env) {
  const override = env && env.SOURCEBOLT_BINARY;
  if (override) return override;
  const target = resolveTarget(platform, arch);
  if (!target) throw new Error(unsupportedMessage(platform, arch));
  try {
    return resolveFn(`${target.pkg}/bin/${target.file}`);
  } catch (e) {
    throw new Error(missingPackageMessage(target.pkg, platform, arch));
  }
}

module.exports = {
  SCOPE,
  NAME,
  SUPPORTED,
  packageName,
  resolveTarget,
  binaryFile,
  unsupportedMessage,
  missingPackageMessage,
  locateBinary,
};
