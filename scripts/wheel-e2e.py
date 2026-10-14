#!/usr/bin/env python3
"""Local install smoke test (no index, no upload): create a venv under a path with spaces,
pip install the wheel matching this machine from a wheels dir, run `sourcebolt --version`.

    python scripts/wheel-e2e.py [--wheels dist/wheels]
"""
import argparse
import platform
import subprocess
import sys
import tempfile
from pathlib import Path


def local_tag() -> str:
    m = platform.machine().lower()
    arch = "arm64" if m in ("arm64", "aarch64") else "amd64"
    if sys.platform.startswith("win"):
        return f"win_{arch}"
    if sys.platform == "darwin":
        return "macosx_11_0_arm64" if arch == "arm64" else "macosx_10_9_x86_64"
    return "manylinux_2_17_aarch64" if arch == "arm64" else "manylinux_2_17_x86_64"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--wheels", default="dist/wheels")
    a = ap.parse_args()
    wheels = [w for w in Path(a.wheels).glob("*.whl") if local_tag() in w.name]
    if not wheels:
        print(f"no wheel for {local_tag()} in {a.wheels}", file=sys.stderr)
        return 1
    with tempfile.TemporaryDirectory(prefix="sb wheel e2e ") as d:
        venv = Path(d) / "my venv"
        subprocess.run([sys.executable, "-m", "venv", str(venv)], check=True)
        bindir = venv / ("Scripts" if sys.platform.startswith("win") else "bin")
        py = bindir / ("python.exe" if sys.platform.startswith("win") else "python")
        subprocess.run([str(py), "-m", "pip", "install", "--no-index", "--quiet", str(wheels[0].resolve())], check=True)
        exe = bindir / ("sourcebolt.exe" if sys.platform.startswith("win") else "sourcebolt")
        out = subprocess.run([str(exe), "--version"], capture_output=True, text=True)
        print(f"console script ({exe}): rc={out.returncode} out={out.stdout.strip()!r}")
        mod = subprocess.run([str(py), "-m", "sourcebolt", "--version"], capture_output=True, text=True)
        print(f"python -m sourcebolt: rc={mod.returncode} out={mod.stdout.strip()!r}")
        bad = subprocess.run([str(exe), "definitely-not-a-command"], capture_output=True, text=True)
        print(f"unknown command rc={bad.returncode}")
        ok = out.returncode == 0 and out.stdout.startswith("sourcebolt ") and mod.stdout == out.stdout and bad.returncode == 2
        return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
