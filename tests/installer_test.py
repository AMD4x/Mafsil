#!/usr/bin/env python3
"""Exercise real installer file operations exclusively in temporary fixtures.

Network is disabled by the --bundle path. No PATH, services or host install.
The fault test alters a disposable copy at the publication point to inject IO
failure; it verifies the actual rollback implementation, not a substitute.
"""
import argparse
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
WINDOWS = sys.platform == "win32"
OPTIONS = None

def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()

class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.fixture = tempfile.TemporaryDirectory(prefix="mafsil-installer-test-")
        self.addCleanup(self.fixture.cleanup)
        self.root = pathlib.Path(self.fixture.name)
        self.bundle = self.root / "bundle"
        shutil.copytree(OPTIONS.bundle, self.bundle)
        self.dest = self.root / "installed space [literal]"
        self.script = ROOT / "scripts" / ("install.ps1" if WINDOWS else "install.sh")

    def run_installer(self, action="Install", success=True, script=None, destination=None, version="v0.1.0", bundle=None):
        destination = destination or self.dest
        script = script or self.script
        bundle = bundle or self.bundle
        if WINDOWS:
            command = ["pwsh.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-File", str(script), "-Action", action, "-Version", version, "-Destination", str(destination), "-BundleDirectory", str(bundle)]
        else:
            command = ["sh", str(script), "--action", action.lower(), "--version", version, "--destination", str(destination), "--bundle", str(bundle)]
        result = subprocess.run(command, capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=45)
        self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)
        return result

    def installed(self):
        return {p.name: digest(p) for p in self.dest.iterdir() if p.is_file()}

    def no_debris(self):
        self.assertFalse(any(p.name.startswith(".mafsil-install") or p.name.endswith(".install-lock") for p in self.root.iterdir()))

    def test_install_idempotent_update_uninstall_preserves_user_files(self):
        self.run_installer()
        self.run_installer("Status")
        (self.dest / "operator-config.json").write_text('{"fixture":"keep"}', encoding="utf-8")
        before = self.installed()
        self.run_installer()
        self.assertEqual(self.installed(), before)
        self.run_installer("Uninstall")
        self.assertEqual(list(self.installed()), ["operator-config.json"])
        self.run_installer("Uninstall")
        self.no_debris()

    def test_corrupt_bundle_never_changes_existing_installation(self):
        self.run_installer()
        before = self.installed()
        (self.bundle / "LICENSE").write_text("corrupt fixture", encoding="utf-8")
        self.run_installer(success=False)
        self.assertEqual(self.installed(), before)
        self.no_debris()

    def test_upgrade_from_older_fixture_preserves_configuration(self):
        # A real native executable simulates an older installed version. It is
        # deliberately a version-probe fixture, not a previously shipped build.
        older = self.root / "older-bundle"
        shutil.copytree(self.bundle, older)
        candidates = list(older.glob("mafsil_v0.1.0_*"))
        import platform
        target = ("windows_amd64.exe" if WINDOWS else "linux_" + ("arm64" if platform.machine().lower() in ("aarch64", "arm64") else "amd64"))
        for candidate in candidates:
            candidate.unlink()
        source = self.root / "older.go"
        source.write_text('package main\nimport "fmt"\nfunc main() { fmt.Println("Mafsil 0.0.9 (upgrade-fixture)") }\n', encoding="utf-8")
        subprocess.run(["go", "build", "-trimpath", "-o", str(older / ("mafsil_v0.0.9_" + target)), str(source)], check=True, cwd=ROOT, timeout=120, env=os.environ | {"CGO_ENABLED": "0"})
        records = [digest(p) + "  " + p.name for p in sorted(older.iterdir()) if p.name != "SHA256SUMS"]
        (older / "SHA256SUMS").write_text("\n".join(records) + "\n", encoding="ascii")
        self.run_installer(version="v0.0.9", bundle=older)
        config = self.dest / "config.json"
        config.write_text('{"workspace":"operator-owned fixture"}', encoding="utf-8")
        before = config.read_bytes()
        scratch = self.dest / "scratch"
        scratch.mkdir()
        (scratch / "keep.txt").write_text("preserve", encoding="utf-8")
        binary = self.dest / ("mafsil.exe" if WINDOWS else "mafsil")
        old_digest = digest(binary)
        self.run_installer()
        self.assertNotEqual(digest(binary), old_digest)
        self.assertEqual(config.read_bytes(), before)
        self.assertEqual((scratch / "keep.txt").read_text(), "preserve")
        status = self.run_installer("Status").stdout
        self.assertIn("v0.1.0", status)
        self.no_debris()

    def test_installed_script_can_uninstall_itself(self):
        self.run_installer()
        self.run_installer("Uninstall", script=self.dest / self.script.name)
        self.assertFalse(self.dest.exists())
        self.no_debris()

    def test_hard_linked_managed_file_is_refused(self):
        self.run_installer()
        external = self.root / "linked-license"
        os.link(self.dest / "LICENSE", external)
        before = self.installed()
        self.run_installer("Uninstall", success=False)
        self.assertEqual(self.installed(), before)
        self.assertEqual(digest(external), before["LICENSE"])
        self.no_debris()

    def test_duplicate_checksum_record_is_refused(self):
        manifest = self.bundle / "SHA256SUMS"
        content = manifest.read_text(encoding="ascii")
        manifest.write_text(content + content.splitlines()[0] + "\n", encoding="ascii")
        self.run_installer(success=False)
        self.assertFalse(self.dest.exists())
        self.no_debris()

    def test_unowned_file_is_preserved(self):
        self.dest.mkdir()
        name = "mafsil.exe" if WINDOWS else "mafsil"
        (self.dest / name).write_bytes(b"unrelated file")
        self.run_installer(success=False)
        self.assertEqual((self.dest / name).read_bytes(), b"unrelated file")
        self.no_debris()

    def test_modified_managed_file_is_preserved(self):
        self.run_installer()
        (self.dest / "LICENSE").write_text("operator modification", encoding="utf-8")
        before = self.installed()
        self.run_installer("Uninstall", success=False)
        self.run_installer(success=False)
        self.assertEqual(self.installed(), before)
        self.no_debris()

    def test_partial_publication_rolls_back(self):
        self.run_installer()
        before = self.installed()
        source = self.script.read_text(encoding="utf-8")
        if WINDOWS:
            needle = "            $changed.Add($name)"
            self.assertEqual(source.count(needle), 1)
            source = source.replace(needle, needle + "\n            throw 'Injected IO failure after publishing first file'")
        else:
            needle = 'changed="$changed $name"; done'
            self.assertEqual(source.count(needle), 1)
            source = source.replace(needle, 'changed="$changed $name"; fail "Injected IO failure"; done')
        copy = self.root / self.script.name
        copy.write_text(source, encoding="utf-8", newline="\n")
        self.run_installer(script=copy, success=False)
        self.assertEqual(self.installed(), before)
        self.no_debris()

    def test_lock_exclusion_and_bad_marker(self):
        lock = pathlib.Path(str(self.dest) + ".install-lock")
        if WINDOWS:
            lock.write_text("fixture lock", encoding="utf-8")
        else:
            lock.mkdir()
        self.run_installer(success=False)
        self.assertFalse(self.dest.exists())
        if WINDOWS:
            lock.unlink()
        else:
            lock.rmdir()
        self.run_installer()
        marker = self.dest / ("mafsil.install.json" if WINDOWS else "mafsil.install")
        marker.write_text("{}", encoding="utf-8")
        before = self.installed()
        self.run_installer("Uninstall", success=False)
        self.assertEqual(self.installed(), before)

    def test_reparse_destination_is_refused(self):
        target = self.root / "outside"
        target.mkdir()
        (target / "keep").write_bytes(b"unrelated")
        if WINDOWS:
            setup = self.root / "junction.ps1"
            setup.write_text("param($LinkPath,$TargetPath)\n$ErrorActionPreference='Stop'\nNew-Item -ItemType Junction -Path $LinkPath -Target $TargetPath | Out-Null\n", encoding="utf-8")
            subprocess.run(["pwsh.exe", "-NoProfile", "-File", str(setup), str(self.dest), str(target)], check=True, capture_output=True)
        else:
            self.dest.symlink_to(target, target_is_directory=True)
        self.run_installer(success=False)
        self.assertEqual([p.name for p in target.iterdir()], ["keep"])
        if WINDOWS:
            os.rmdir(self.dest)
        else:
            self.dest.unlink()

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--bundle", type=pathlib.Path, required=True)
    OPTIONS, rest = parser.parse_known_args()
    unittest.main(argv=[sys.argv[0]] + rest)
