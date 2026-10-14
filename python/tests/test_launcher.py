import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from sourcebolt import _launcher as L  # noqa: E402


class FindBinaryTest(unittest.TestCase):
    def test_binary_name(self):
        self.assertEqual(L.binary_name("nt"), "sourcebolt.exe")
        self.assertEqual(L.binary_name("posix"), "sourcebolt")

    def test_finds_bundled_binary_in_path_with_spaces(self):
        with tempfile.TemporaryDirectory(prefix="sb dir ") as d:
            pkg = Path(d) / "site packages" / "sourcebolt"
            (pkg / "bin").mkdir(parents=True)
            exe = pkg / "bin" / "sourcebolt.exe"
            exe.write_bytes(b"x")
            got = L.find_binary(pkg, env={}, os_name="nt")
            self.assertEqual(got, exe)

    def test_missing_binary_message(self):
        with tempfile.TemporaryDirectory() as d:
            with self.assertRaises(FileNotFoundError) as cm:
                L.find_binary(Path(d), env={}, os_name="posix")
            self.assertIn("does not match your platform", str(cm.exception))

    def test_env_override(self):
        with tempfile.TemporaryDirectory(prefix="sb dir ") as d:
            f = Path(d) / "my bin"
            f.write_bytes(b"x")
            self.assertEqual(L.find_binary(Path(d), env={L.ENV_OVERRIDE: str(f)}), f)
            with self.assertRaises(FileNotFoundError):
                L.find_binary(Path(d), env={L.ENV_OVERRIDE: str(f) + ".nope"})


class RunTest(unittest.TestCase):
    def test_unix_uses_execv_with_argv0_and_args(self):
        calls = []
        L.run(Path("/opt/my dir/sourcebolt"), ["serve", "--x"], os_name="posix",
              execv=lambda p, a: calls.append((p, a)))
        self.assertEqual(calls, [(str(Path("/opt/my dir/sourcebolt")), [str(Path("/opt/my dir/sourcebolt")), "serve", "--x"])])

    def test_windows_returns_exit_code_and_passes_list_argv(self):
        seen = {}

        class R:
            returncode = 9

        def fake(argv):
            seen["argv"] = argv
            return R()

        code = L.run(Path("C:/Program Files/sb/sourcebolt.exe"), ["a b", "c"], os_name="nt", runner=fake)
        self.assertEqual(code, 9)
        self.assertEqual(seen["argv"][1:], ["a b", "c"])
        self.assertIn("Program Files", seen["argv"][0])

    def test_windows_keyboard_interrupt(self):
        def boom(argv):
            raise KeyboardInterrupt

        self.assertEqual(L.run(Path("x"), [], os_name="nt", runner=boom), 130)

    @unittest.skipUnless(os.name == "nt", "real subprocess exit-code path is the Windows branch")
    def test_real_subprocess_exit_code(self):
        code = L.run(Path(sys.executable), ["-c", "raise SystemExit(5)"])
        self.assertEqual(code, 5)


if __name__ == "__main__":
    unittest.main()
