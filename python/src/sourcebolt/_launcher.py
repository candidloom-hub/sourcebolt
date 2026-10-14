"""Locate and run the bundled sourcebolt binary."""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path
from typing import Callable, Mapping, Optional, Sequence

ENV_OVERRIDE = "SOURCEBOLT_BINARY"


def binary_name(os_name: Optional[str] = None) -> str:
    return "sourcebolt.exe" if (os_name or os.name) == "nt" else "sourcebolt"


def find_binary(
    package_dir: Optional[Path] = None,
    env: Optional[Mapping[str, str]] = None,
    os_name: Optional[str] = None,
) -> Path:
    """Return the path of the bundled binary.

    Resolution order: $SOURCEBOLT_BINARY, then <package dir>/bin/sourcebolt[.exe].
    Raises FileNotFoundError with a user-ready message when nothing is found.
    """
    env = os.environ if env is None else env
    override = env.get(ENV_OVERRIDE)
    if override:
        p = Path(override)
        if not p.is_file():
            raise FileNotFoundError(f"{ENV_OVERRIDE}={override} does not exist")
        return p
    base = Path(package_dir) if package_dir is not None else Path(__file__).resolve().parent
    p = base / "bin" / binary_name(os_name)
    if not p.is_file():
        raise FileNotFoundError(
            f"sourcebolt binary not found at {p}. This wheel does not match your platform; "
            "install a release binary from https://github.com/candidloom-hub/sourcebolt instead."
        )
    return p


def run(
    binary: Path,
    args: Sequence[str],
    os_name: Optional[str] = None,
    execv: Callable[[str, list], None] = os.execv,
    runner: Callable[..., "subprocess.CompletedProcess"] = subprocess.run,
) -> int:
    """Unix: replace this process (no lingering Python parent). Windows: spawn and return the exit code.

    The binary path and every argument are passed as separate list items, never through a shell,
    so paths containing spaces are safe.
    """
    argv = [str(binary), *args]
    if (os_name or os.name) != "nt":
        execv(str(binary), argv)  # does not return on success
        return 1
    try:
        return runner(argv).returncode
    except KeyboardInterrupt:
        return 130


def main() -> None:
    try:
        binary = find_binary()
    except FileNotFoundError as e:
        sys.stderr.write(f"sourcebolt: {e}\n")
        sys.exit(1)
    sys.exit(run(binary, sys.argv[1:]))
