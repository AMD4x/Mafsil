#!/usr/bin/env python3
"""Release assembly must bind every native candidate to the requested commit."""
import hashlib
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
COMMIT = "a" * 40
TARGETS = ("windows_amd64", "linux_amd64", "linux_arm64")


class AssemblyTests(unittest.TestCase):
    def setUp(self):
        self.fixture = tempfile.TemporaryDirectory(prefix="mafsil-assembly-")
        self.addCleanup(self.fixture.cleanup)
        self.root = pathlib.Path(self.fixture.name)
        self.inputs = self.root / "input"
        for target in TARGETS:
            folder = self.inputs / ("candidate-" + target)
            folder.mkdir(parents=True)
            for name in ("LICENSE", "THIRD_PARTY_NOTICES.md", "install.ps1", "install.sh", "DEPENDENCIES.json"):
                (folder / name).write_bytes(b"shared fixture")
            binary = "mafsil_v0.1.0_" + target + (".exe" if target.startswith("windows") else "")
            (folder / binary).write_bytes(b"fixture binary " + target.encode())
            info = {"commit": COMMIT, "version": "v0.1.0", "target": target.replace("_", "/")}
            (folder / ("BUILD-INFO_" + target + ".json")).write_text(json.dumps(info), encoding="utf-8")
            self.manifest(folder)

    def manifest(self, folder):
        records = [hashlib.sha256(p.read_bytes()).hexdigest() + "  " + p.name for p in sorted(folder.iterdir()) if p.name != "SHA256SUMS"]
        (folder / "SHA256SUMS").write_text("\n".join(records) + "\n", encoding="ascii")

    def assemble(self, success, expected_manifest=None):
        command = [sys.executable, str(ROOT / "scripts/assemble.py"), "--input", str(self.inputs), "--output", str(self.root / "output"), "--commit", COMMIT, "--version", "v0.1.0"]
        if expected_manifest is not None:
            command.extend(["--expected-manifest-sha256", expected_manifest])
        result = subprocess.run(command, capture_output=True, text=True)
        self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)
        return self.root / "output"

    def test_complete_candidates(self):
        output = self.assemble(True)
        self.assertEqual(len(list(output.iterdir())), 12)
        for line in (output / "SHA256SUMS").read_text().splitlines():
            checksum, name = line.split("  ")
            self.assertEqual(hashlib.sha256((output / name).read_bytes()).hexdigest(), checksum)

    def test_corrupt_payload(self):
        (self.inputs / "candidate-linux_arm64/LICENSE").write_bytes(b"corruption")
        output = self.assemble(False)
        self.assertEqual(list(output.iterdir()), [])

    def expected_manifest(self):
        checksums = {}
        for folder in self.inputs.iterdir():
            for line in (folder / "SHA256SUMS").read_text().splitlines():
                checksum, name = line.split("  ")
                checksums[name] = checksum
        return "".join(f"{checksums[name]}  {name}\n" for name in sorted(checksums)).encode("ascii")

    def test_expected_manifest_matches_exact_assets(self):
        expected = self.expected_manifest()
        output = self.assemble(True, hashlib.sha256(expected).hexdigest())
        self.assertEqual((output / "SHA256SUMS").read_bytes(), expected)

    def test_changed_assets_fail_manifest_verification(self):
        expected_digest = hashlib.sha256(self.expected_manifest()).hexdigest()
        folder = self.inputs / "candidate-linux_arm64"
        (folder / "mafsil_v0.1.0_linux_arm64").write_bytes(b"different but internally valid candidate")
        self.manifest(folder)
        output = self.assemble(False, expected_digest)
        self.assertEqual(list(output.iterdir()), [])

    def test_wrong_commit_even_with_valid_checksum(self):
        folder = self.inputs / "candidate-linux_amd64"
        path = folder / "BUILD-INFO_linux_amd64.json"
        info = json.loads(path.read_text())
        info["commit"] = "b" * 40
        path.write_text(json.dumps(info), encoding="utf-8")
        self.manifest(folder)
        self.assemble(False)

    def test_common_payload_disagreement(self):
        folder = self.inputs / "candidate-linux_arm64"
        (folder / "install.sh").write_bytes(b"different script")
        self.manifest(folder)
        self.assemble(False)

    def test_duplicate_manifest_entry(self):
        path = self.inputs / "candidate-windows_amd64/SHA256SUMS"
        text = path.read_text()
        path.write_text(text + text.splitlines()[0] + "\n")
        self.assemble(False)


if __name__ == "__main__":
    unittest.main()
