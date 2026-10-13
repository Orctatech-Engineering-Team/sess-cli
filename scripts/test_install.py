#!/usr/bin/env python3
"""Exercise the installer with local release archives and isolated homes."""

import hashlib
import io
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest


INSTALLER = Path(__file__).with_name("install.sh").resolve()


class InstallTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.home = self.root / "home"
        self.home.mkdir()
        self.assets = self.root / "assets"
        self.assets.mkdir()
        os_name = {"Linux": "linux", "Darwin": "darwin"}[os.uname().sysname]
        arch = {"x86_64": "amd64", "aarch64": "arm64", "arm64": "arm64"}[os.uname().machine]
        archive = self.assets / f"sess-{os_name}-{arch}.tar.gz"
        with tarfile.open(archive, "w:gz") as tar:
            binary = b"#!/bin/sh\nprintf 'SESS fixture-new\\n'\n"
            entry = tarfile.TarInfo("sess")
            entry.size, entry.mode = len(binary), 0o755
            tar.addfile(entry, io.BytesIO(binary))
        self.checksum = f"{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n"
        (self.assets / "checksums.txt").write_text(self.checksum)
        self.tools = self.root / "tools"
        self.tools.mkdir()
        for name in ("bash", "curl", "tar", "gzip", "uname", "id", "mkdir", "mktemp", "rm",
                     "awk", "sha256sum", "shasum", "install", "mv", "dirname"):
            command = shutil.which(name)
            if command:
                (self.tools / name).symlink_to(command)
        self.env = {k: v for k, v in os.environ.items() if not k.startswith("SESS_")}
        self.env.update(HOME=str(self.home), PATH=str(self.tools),
                        SESS_BASE_URL=self.assets.as_uri())

    def run_installer(self, **env):
        return subprocess.run([shutil.which("bash"), str(INSTALLER)],
                              env=self.env | env, text=True, capture_output=True)

    def old_binary(self, directory):
        directory.mkdir(parents=True, exist_ok=True)
        binary = directory / "sess"
        binary.write_text("#!/bin/sh\nprintf 'old\\n'\n")
        binary.chmod(0o755)
        return binary

    def test_new_install_creates_user_bin_and_preserves_state(self):
        state = self.home / ".sess-cli" / "sess.db"
        state.parent.mkdir()
        state.write_bytes(b"existing database")
        result = self.run_installer()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((self.home / ".local/bin/sess").exists())
        self.assertIn("PATH", result.stdout)
        self.assertEqual(state.read_bytes(), b"existing database")

    def test_updates_existing_path_in_place(self):
        directory = self.root / "custom bin"
        old = self.old_binary(directory)
        result = self.run_installer(PATH=f"{directory}:{self.tools}")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("fixture-new", old.read_text())
        self.assertFalse((self.home / ".local/bin/sess").exists())

    def test_repeated_user_install_updates_without_path_entry(self):
        for _ in range(2):
            result = self.run_installer()
            self.assertEqual(result.returncode, 0, result.stderr)
        directory = self.home / ".local/bin"
        self.assertIn("fixture-new", (directory / "sess").read_text())
        self.assertEqual([p.name for p in directory.iterdir()], ["sess"])

    def test_failed_staging_preserves_existing_binary(self):
        old = self.old_binary(self.root / "old")
        (self.tools / "install").unlink()
        tool = self.tools / "install"
        tool.write_text("#!/bin/sh\nexit 1\n")
        tool.chmod(0o755)
        result = self.run_installer(PATH=f"{old.parent}:{self.tools}")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("old", old.read_text())
        self.assertEqual([p.name for p in old.parent.iterdir()], ["sess"])

    def test_explicit_directory_and_shadow_warning(self):
        old = self.old_binary(self.root / "old")
        directory = self.root / "new bin"
        result = self.run_installer(SESS_INSTALL_DIR=str(directory),
                                    PATH=f"{old.parent}:{self.tools}")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("another installation", result.stdout)
        self.assertIn("old", old.read_text())
        self.assertTrue((directory / "sess").exists())

    def test_bad_checksum_preserves_existing_binary(self):
        old = self.old_binary(self.root / "old")
        (self.assets / "checksums.txt").write_text(self.checksum.replace(self.checksum[:64], "0" * 64))
        result = self.run_installer(PATH=f"{old.parent}:{self.tools}")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("checksum mismatch", result.stderr)
        self.assertIn("old", old.read_text())

    def test_symlink_is_not_replaced(self):
        target = self.old_binary(self.root / "managed")
        directory = self.root / "linked"
        directory.mkdir()
        (directory / "sess").symlink_to(target)
        result = self.run_installer(PATH=f"{directory}:{self.tools}")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("symlink", result.stderr)
        self.assertIn("old", target.read_text())

    @unittest.skipIf(os.getuid() == 0, "permission checks require a non-root user")
    def test_unwritable_existing_install_does_not_create_second_copy(self):
        directory = self.root / "system"
        old = self.old_binary(directory)
        directory.chmod(0o555)
        self.addCleanup(directory.chmod, 0o755)
        result = self.run_installer(PATH=f"{directory}:{self.tools}")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("SESS_INSTALL_DIR", result.stderr)
        self.assertIn("old", old.read_text())
        self.assertFalse((self.home / ".local/bin/sess").exists())


if __name__ == "__main__":
    unittest.main()
