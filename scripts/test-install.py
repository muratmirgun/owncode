#!/usr/bin/env python3
"""Exercise the installer without network access or changes to the user's home."""
import hashlib
import io
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


class InstallerTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.bin = self.root / "commands"
        self.bin.mkdir()
        self.destination = self.root / "install with spaces"
        self.env = dict(os.environ, HOME=str(self.root), FIXTURE=str(self.root),
                        PATH=f"{self.bin}:{os.environ['PATH']}")
        self.env.pop("VERSION", None)
        self.env.pop("OWNCODE_INSTALL_DIR", None)
        self.command("uname", '#!/bin/sh\ncase "$1" in -s) echo Darwin;; -m) echo arm64;; esac\n')
        self.command("curl", '''#!/usr/bin/env python3
import os, pathlib, shutil, sys
args = sys.argv[1:]
url = next(a for a in args if a.startswith("https://"))
if os.environ.get("FAIL_DOWNLOAD"):
    sys.exit(22)
if url.endswith("/latest"):
    print('{"tag_name":"v0.1.0"}')
else:
    destination = args[args.index("-o") + 1]
    shutil.copyfile(pathlib.Path(os.environ["FIXTURE"]) / url.rsplit("/", 1)[-1], destination)
''')
        archive = self.root / "owncode-mac-arm64.tar.gz"
        with tarfile.open(archive, "w:gz") as output:
            data = b"#!/bin/sh\necho 0.1.0\n"
            info = tarfile.TarInfo("owncode")
            info.size = len(data)
            info.mode = 0o755
            output.addfile(info, io.BytesIO(data))
        digest = hashlib.sha256(archive.read_bytes()).hexdigest()
        (self.root / "checksums.txt").write_text(f"{digest}  {archive.name}\n")

    def command(self, name, content):
        path = self.bin / name
        path.write_text(content)
        path.chmod(0o755)

    def run_installer(self, *args):
        return subprocess.run(["bash", str(ROOT / "install"), "--install-dir", str(self.destination), *args],
                              env=self.env, capture_output=True, text=True)

    def test_latest_and_pinned_install(self):
        for args in [(), ("--version", "v0.1.0"), ("--version", "0.1.0")]:
            result = self.run_installer(*args)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(subprocess.check_output([str(self.destination / "owncode"), "--version"], text=True).strip(), "0.1.0")
        self.assertFalse((self.root / ".zshrc").exists())
        self.assertEqual(list(self.destination.glob(".owncode-install.*")), [])

    def test_bad_checksum_keeps_existing_binary(self):
        self.destination.mkdir()
        binary = self.destination / "owncode"
        binary.write_text("existing")
        (self.root / "checksums.txt").write_text("0" * 64 + "  owncode-mac-arm64.tar.gz\n")
        result = self.run_installer("--version", "v0.1.0")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Checksum mismatch", result.stderr)
        self.assertEqual(binary.read_text(), "existing")

    def test_download_failure_does_not_install(self):
        self.env["FAIL_DOWNLOAD"] = "1"
        self.assertNotEqual(self.run_installer().returncode, 0)
        self.assertFalse(self.destination.exists())

    def test_unsupported_platform_and_invalid_version(self):
        self.assertNotEqual(self.run_installer("--version", "../../bad").returncode, 0)
        self.command("uname", "#!/bin/sh\necho unsupported\n")
        self.assertNotEqual(self.run_installer().returncode, 0)
        self.assertFalse(self.destination.exists())

    def test_formula_requires_all_archives(self):
        checksums = self.root / "formula-checksums.txt"
        output = self.root / "owncode.rb"
        command = ["python3", str(ROOT / "scripts/homebrew-formula.py"), "v0.1.0", str(checksums), str(output)]
        checksums.write_text("a" * 64 + "  owncode-mac-arm64.tar.gz\n")
        self.assertNotEqual(subprocess.run(command, capture_output=True).returncode, 0)
        checksums.write_text("".join("a" * 64 + f"  owncode-{os_name}-{arch}.tar.gz\n"
                                     for os_name in ["mac", "linux"] for arch in ["arm64", "x86_64"]))
        self.assertEqual(subprocess.run(command, capture_output=True).returncode, 0)
        self.assertEqual(output.read_text().count('sha256 "'), 4)


if __name__ == "__main__":
    unittest.main()
