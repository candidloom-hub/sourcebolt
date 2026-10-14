import importlib.util
import tempfile
import unittest
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("build_wheels", ROOT / "scripts" / "build-wheels.py")
bw = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bw)


def fake_dist(d: Path) -> None:
    for sub, exe in [("sourcebolt_windows_amd64_v1", "sourcebolt.exe"),
                     ("sourcebolt_linux_arm64_v8.0", "sourcebolt"),
                     ("sourcebolt-browser_linux_amd64_v1", "sourcebolt")]:
        (d / sub).mkdir(parents=True)
        (d / sub / exe).write_bytes(b"BIN")


class BuildWheelsTest(unittest.TestCase):
    def test_pep440(self):
        self.assertEqual(bw.pep440("v0.1.0"), "0.1.0")
        self.assertEqual(bw.pep440("1.2.3-rc.1"), "1.2.3rc1")
        self.assertEqual(bw.pep440("1.2.3-beta2"), "1.2.3b2")
        with self.assertRaises(ValueError):
            bw.pep440("latest")

    def test_find_binary_ignores_browser_build(self):
        with tempfile.TemporaryDirectory(prefix="sb dist ") as d:
            d = Path(d)
            fake_dist(d)
            lin = [p for p in bw.PLATFORMS if p["goos"] == "linux" and p["goarch"] == "amd64"][0]
            self.assertIsNone(bw.find_binary(d, lin))
            arm = [p for p in bw.PLATFORMS if p["goos"] == "linux" and p["goarch"] == "arm64"][0]
            self.assertTrue(bw.find_binary(d, arm).name == "sourcebolt")

    def test_wheel_layout_tags_and_modes(self):
        with tempfile.TemporaryDirectory(prefix="sb dist ") as d:
            d = Path(d)
            fake_dist(d)
            self.assertEqual(bw.main(["--version", "v0.1.0", "--dist", str(d),
                                      "--platforms", "win_amd64,manylinux_2_17_aarch64"]), 0)
            wheels = sorted((d / "wheels").glob("*.whl"))
            names = [w.name for w in wheels]
            self.assertIn("sourcebolt-0.1.0-py3-none-win_amd64.whl", names)
            self.assertIn("sourcebolt-0.1.0-py3-none-manylinux_2_17_aarch64.manylinux2014_aarch64.musllinux_1_2_aarch64.whl", names)
            win = d / "wheels" / "sourcebolt-0.1.0-py3-none-win_amd64.whl"
            with zipfile.ZipFile(win) as z:
                n = z.namelist()
                self.assertIn("sourcebolt/bin/sourcebolt.exe", n)
                self.assertIn("sourcebolt/_launcher.py", n)
                self.assertIn("sourcebolt-0.1.0.dist-info/RECORD", n)
                self.assertIn("Root-Is-Purelib: false", z.read("sourcebolt-0.1.0.dist-info/WHEEL").decode())
                self.assertIn("sourcebolt = sourcebolt._launcher:main",
                              z.read("sourcebolt-0.1.0.dist-info/entry_points.txt").decode())
                self.assertIn("Name: sourcebolt", z.read("sourcebolt-0.1.0.dist-info/METADATA").decode())
            lin = [w for w in wheels if "manylinux" in w.name][0]
            with zipfile.ZipFile(lin) as z:
                mode = z.getinfo("sourcebolt/bin/sourcebolt").external_attr >> 16
                self.assertEqual(mode & 0o777, 0o755)

    def test_missing_binary_is_an_error_unless_allowed(self):
        with tempfile.TemporaryDirectory() as d:
            self.assertEqual(bw.main(["--version", "0.1.0", "--dist", d, "--platforms", "win_amd64"]), 1)
            self.assertEqual(bw.main(["--version", "0.1.0", "--dist", d, "--platforms", "win_amd64",
                                      "--allow-missing"]), 0)


if __name__ == "__main__":
    unittest.main()
