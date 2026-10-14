#!/usr/bin/env python3
"""Build one platform wheel per GoReleaser binary in dist/ (no publishing).

    python scripts/build-wheels.py --version 0.1.0 [--dist dist] [--out dist/wheels]
                                   [--platforms win_amd64,...] [--allow-missing]

Wheels are tagged py3-none-<platform>; the Go binary is static (CGO_ENABLED=0), so the
Linux wheels carry both manylinux and musllinux tags. Metadata comes from
python/pyproject.toml.tmpl. The wheel is assembled directly (zipfile) so the platform
tag and the executable bit are exact and no build backend is needed.
"""
from __future__ import annotations

import argparse
import base64
import hashlib
import re
import sys
import tomllib
import zipfile
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
PY_SRC = REPO / "python" / "src" / "sourcebolt"
TEMPLATE = REPO / "python" / "pyproject.toml.tmpl"

# goos, goarch -> wheel platform tag (compound tags use '.', per PEP 425 / wheel filename spec)
LINUX_X64 = "manylinux_2_17_x86_64.manylinux2014_x86_64.musllinux_1_2_x86_64"
LINUX_ARM64 = "manylinux_2_17_aarch64.manylinux2014_aarch64.musllinux_1_2_aarch64"
PLATFORMS = [
    {"goos": "linux", "goarch": "amd64", "tag": LINUX_X64, "exe": "sourcebolt"},
    {"goos": "linux", "goarch": "arm64", "tag": LINUX_ARM64, "exe": "sourcebolt"},
    {"goos": "darwin", "goarch": "amd64", "tag": "macosx_10_9_x86_64", "exe": "sourcebolt"},
    {"goos": "darwin", "goarch": "arm64", "tag": "macosx_11_0_arm64", "exe": "sourcebolt"},
    {"goos": "windows", "goarch": "amd64", "tag": "win_amd64", "exe": "sourcebolt.exe"},
    {"goos": "windows", "goarch": "arm64", "tag": "win_arm64", "exe": "sourcebolt.exe"},
]


def pep440(version: str) -> str:
    """Convert semver (optionally v-prefixed, -rc.1 style prerelease) to PEP 440."""
    v = version.lstrip("v")
    m = re.fullmatch(r"(\d+\.\d+\.\d+)(?:-(alpha|beta|rc|a|b)\.?(\d+))?", v)
    if not m:
        raise ValueError(f"unsupported version {version!r}")
    base, pre, n = m.groups()
    if not pre:
        return base
    return f"{base}{ {'alpha': 'a', 'beta': 'b'}.get(pre, pre) }{n}"


def find_binary(dist: Path, spec: dict) -> Path | None:
    """GoReleaser layout: dist/sourcebolt_<goos>_<goarch>[_v1|_v8.0]/<exe>. Ignores the browser build."""
    prefix = f"sourcebolt_{spec['goos']}_{spec['goarch']}"
    if not dist.is_dir():
        return None
    for d in sorted(dist.iterdir()):
        if d.is_dir() and (d.name == prefix or d.name.startswith(prefix + "_")):
            f = d / spec["exe"]
            if f.is_file():
                return f
    return None


def load_metadata(version: str) -> dict:
    text = TEMPLATE.read_text(encoding="utf8").replace("@VERSION@", version)
    return tomllib.loads(text)["project"]


def render_metadata(project: dict, long_description: str | None) -> str:
    lines = [
        "Metadata-Version: 2.1",
        f"Name: {project['name']}",
        f"Version: {project['version']}",
        f"Summary: {project['description']}",
        f"License: {project['license']['text']}",
        f"Requires-Python: {project['requires-python']}",
    ]
    if project.get("keywords"):
        lines.append("Keywords: " + ",".join(project["keywords"]))
    lines += [f"Classifier: {c}" for c in project.get("classifiers", [])]
    lines += [f"Project-URL: {k}, {v}" for k, v in project.get("urls", {}).items()]
    # Always ship a markdown description (README.md when present) so PyPI renders it.
    body = long_description or project["description"]
    lines += ["Description-Content-Type: text/markdown", "", body]
    return "\n".join(lines) + "\n"


def _hash(data: bytes) -> str:
    return "sha256=" + base64.urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b"=").decode()


def build_wheel(binary: Path, spec: dict, version: str, out: Path) -> Path:
    project = load_metadata(version)
    name = project["name"]
    dist_info = f"{name}-{version}.dist-info"
    readme = REPO / "README.md"
    long_desc = readme.read_text(encoding="utf8") if readme.is_file() else None

    entries: list[tuple[str, bytes, int]] = []  # (arcname, data, unix mode)
    for f in sorted(PY_SRC.glob("*.py")):
        entries.append((f"{name}/{f.name}", f.read_bytes(), 0o644))
    entries.append((f"{name}/bin/{spec['exe']}", binary.read_bytes(), 0o755))
    for lic in ("LICENSE", "NOTICE"):
        p = REPO / lic
        if p.is_file():
            entries.append((f"{dist_info}/{lic}", p.read_bytes(), 0o644))
    entries.append((f"{dist_info}/METADATA", render_metadata(project, long_desc).encode("utf8"), 0o644))
    wheel_txt = (
        "Wheel-Version: 1.0\nGenerator: sourcebolt build-wheels\nRoot-Is-Purelib: false\n"
        + "".join(f"Tag: py3-none-{t}\n" for t in spec["tag"].split("."))
    )
    entries.append((f"{dist_info}/WHEEL", wheel_txt.encode(), 0o644))
    scripts = "".join(f"{k} = {v}\n" for k, v in project.get("scripts", {}).items())
    entries.append((f"{dist_info}/entry_points.txt", f"[console_scripts]\n{scripts}".encode(), 0o644))
    entries.append((f"{dist_info}/top_level.txt", f"{name}\n".encode(), 0o644))

    record = "".join(f"{a},{_hash(d)},{len(d)}\n" for a, d, _ in entries) + f"{dist_info}/RECORD,,\n"
    entries.append((f"{dist_info}/RECORD", record.encode(), 0o644))

    out.mkdir(parents=True, exist_ok=True)
    wheel = out / f"{name}-{version}-py3-none-{spec['tag']}.whl"
    with zipfile.ZipFile(wheel, "w", zipfile.ZIP_DEFLATED) as z:
        for arc, data, mode in entries:
            zi = zipfile.ZipInfo(arc, date_time=(2020, 1, 1, 0, 0, 0))  # reproducible
            zi.external_attr = (0o100000 | mode) << 16
            zi.compress_type = zipfile.ZIP_DEFLATED
            z.writestr(zi, data)
    return wheel


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--version", required=True)
    ap.add_argument("--dist", default="dist")
    ap.add_argument("--out", default=None)
    ap.add_argument("--platforms", default=None, help="comma-separated wheel tags to build (default: all)")
    ap.add_argument("--allow-missing", action="store_true")
    a = ap.parse_args(argv)

    version = pep440(a.version)
    dist = Path(a.dist)
    out = Path(a.out) if a.out else dist / "wheels"
    wanted = PLATFORMS
    if a.platforms:
        tags = a.platforms.split(",")
        wanted = [p for p in PLATFORMS if p["tag"] in tags or p["tag"].split(".")[0] in tags]
        if len(wanted) != len(tags):
            print(f"build-wheels: unknown platform in {a.platforms}", file=sys.stderr)
            return 1

    built = 0
    for spec in wanted:
        binary = find_binary(dist, spec)
        if binary is None:
            msg = f"no binary for {spec['goos']}/{spec['goarch']} in {dist}"
            if a.allow_missing:
                print(f"warning: {msg}; skipping", file=sys.stderr)
                continue
            print(f"build-wheels: {msg}", file=sys.stderr)
            return 1
        print(build_wheel(binary, spec, version, out))
        built += 1
    print(f"built {built} wheel(s) in {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
