#!/usr/bin/env python3
"""Offline bootstrap tests: fake network responses, real verification/delegation."""
import hashlib
import os
import pathlib
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
WINDOWS = os.name == "nt"
TAG = "v9.8.7"


class BootstrapTests(unittest.TestCase):
    def setUp(self):
        self.fixture = tempfile.TemporaryDirectory(prefix="mafsil-bootstrap-test-")
        self.addCleanup(self.fixture.cleanup)
        self.root = pathlib.Path(self.fixture.name)
        self.dest = self.root / "destination space [literal]"
        self.network = self.root / "network"
        self.network.mkdir()
        self.scratch = self.root / "scratch"
        self.scratch.mkdir()
        self.name = "install.ps1" if WINDOWS else "install.sh"
        payload = (
            "param([string]$Version,[string]$Destination)\n"
            "[IO.Directory]::CreateDirectory($Destination) | Out-Null\n"
            "[IO.File]::WriteAllText((Join-Path $Destination 'received-version'),$Version)\n"
            if WINDOWS else
            '#!/bin/sh\nset -eu\nversion=$2\ndestination=$4\nmkdir -p -- "$destination"\nprintf %s "$version" > "$destination/received-version"\n'
        )
        (self.network / self.name).write_text(payload, encoding="utf-8", newline="\n")
        checksum = hashlib.sha256((self.network / self.name).read_bytes()).hexdigest()
        self.manifest = self.network / "SHA256SUMS"
        self.manifest.write_text(f"{checksum}  {self.name}\n", encoding="ascii")
        self.env = os.environ | {
            "MAFSIL_FIXTURE": str(self.network), "MAFSIL_FIXTURE_TAG": TAG,
            "MAFSIL_FIXTURE_LOCATION": "https://github.com/AMD4x/Mafsil/releases/tag/" + TAG,
            "TEMP": str(self.scratch), "TMP": str(self.scratch), "TMPDIR": str(self.scratch),
            "MAFSIL_FIXTURE_FAIL": "0",
        }
        source = (ROOT / self.name).read_text(encoding="utf-8")
        self.script = self.root / self.name
        if WINDOWS:
            start = source.index("function Get-MafsilLatestLocation")
            end = source.index("function Resolve-MafsilVersion", start)
            source = source[:start] + '''function Get-MafsilLatestLocation {
    [IO.File]::AppendAllText((Join-Path $env:MAFSIL_FIXTURE 'latest-calls'), '1')
    if ($env:MAFSIL_FIXTURE_FAIL -eq '1') { throw 'Injected network failure' }
    return $env:MAFSIL_FIXTURE_LOCATION
}

''' + source[end:]
            start = source.index("function Receive-MafsilFile")
            end = source.index("function Get-MafsilInstallerDigest", start)
            source = source[:start] + '''function Receive-MafsilFile([string]$Uri,[string]$Path,[long]$Limit) {
    if ($env:MAFSIL_FIXTURE_FAIL -eq '1') { throw 'Injected network failure' }
    $base = "https://github.com/AMD4x/Mafsil/releases/download/$env:MAFSIL_FIXTURE_TAG/"
    if (-not $Uri.StartsWith($base)) { throw 'Unexpected asset release' }
    $name = $Uri.Substring($base.Length)
    if ($name -notin @('SHA256SUMS','install.ps1')) { throw 'Unexpected asset' }
    [IO.File]::Copy((Join-Path $env:MAFSIL_FIXTURE $name),$Path,$false)
}

''' + source[end:]
        else:
            tools = self.root / "tools"
            tools.mkdir()
            curl = tools / "curl"
            curl.write_text('''#!/usr/bin/env python3
import os, pathlib, shutil, sys
root=pathlib.Path(os.environ['MAFSIL_FIXTURE'])
if os.environ['MAFSIL_FIXTURE_FAIL']=='1': sys.exit(22)
args=sys.argv[1:]
url=next(a for a in args if a.startswith('https://'))
if url.endswith('/releases/latest'):
    with (root/'latest-calls').open('a') as f: f.write('1')
    sys.stdout.write(os.environ['MAFSIL_FIXTURE_LOCATION'])
else:
    base='https://github.com/AMD4x/Mafsil/releases/download/'+os.environ['MAFSIL_FIXTURE_TAG']+'/'
    assert url.startswith(base), url
    name=url[len(base):]
    assert name in ('SHA256SUMS','install.sh')
    shutil.copyfile(root/name,args[args.index('-o')+1])
''', encoding="utf-8", newline="\n")
            curl.chmod(0o755)
            self.env["PATH"] = str(tools) + os.pathsep + self.env["PATH"]
        self.script.write_text(source, encoding="utf-8", newline="\n")

    def run_bootstrap(self, version=None, success=True):
        if WINDOWS:
            command = ["pwsh.exe", "-NoProfile", "-File", str(self.script), "-Destination", str(self.dest)]
            if version is not None: command += ["-Version", version]
        else:
            command = ["sh", str(self.script), "--destination", str(self.dest)]
            if version is not None: command += ["--version", version]
        result = subprocess.run(command, env=self.env, capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)
        self.assertEqual(list(self.scratch.iterdir()), [], "Bootstrap staging was not cleaned")
        if not success: self.assertFalse(self.dest.exists(), "Unverified installer executed")

    def test_latest_is_resolved_once_and_verified_before_execution(self):
        self.run_bootstrap()
        self.assertEqual((self.dest / "received-version").read_text(), TAG)
        self.assertEqual((self.network / "latest-calls").read_text(), "1")

    def test_explicit_version_does_not_query_latest(self):
        self.run_bootstrap(version=TAG)
        self.assertFalse((self.network / "latest-calls").exists())

    def test_corrupt_installer_is_never_executed(self):
        with (self.network / self.name).open("a") as f: f.write("\n# corruption\n")
        self.run_bootstrap(success=False)

    def test_duplicate_manifest_entry_is_rejected(self):
        text = self.manifest.read_text()
        self.manifest.write_text(text + text)
        self.run_bootstrap(success=False)

    def test_missing_installer_digest_is_rejected(self):
        self.manifest.write_text("0" * 64 + "  LICENSE\n")
        self.run_bootstrap(success=False)

    def test_network_failure_is_not_success(self):
        self.env["MAFSIL_FIXTURE_FAIL"] = "1"
        self.run_bootstrap(success=False)

    def test_unexpected_latest_location_is_rejected(self):
        self.env["MAFSIL_FIXTURE_LOCATION"] = "https://example.invalid/releases/tag/v9.8.7"
        self.run_bootstrap(success=False)

    def test_prerelease_is_not_selected_as_stable(self):
        self.env["MAFSIL_FIXTURE_LOCATION"] += "-preview"
        self.run_bootstrap(success=False)

    def test_invalid_version_is_rejected(self):
        self.run_bootstrap(version="v1.2.3\nextra", success=False)

    @unittest.skipIf(WINDOWS, "POSIX streamed bootstrap behavior")
    def test_partial_download_cannot_start_installation(self):
        source = self.script.read_text()
        truncated = source[:source.index("    stage=$(mktemp")]
        result = subprocess.run(["sh"], input=truncated, env=self.env, capture_output=True, text=True, timeout=10)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.dest.exists())
        self.assertFalse((self.network / "latest-calls").exists())


if __name__ == "__main__":
    unittest.main()
