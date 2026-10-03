#!/usr/bin/env python3
"""Native CI only: public bootstrap, README setup and update in a temporary home."""
import json
import os
import pathlib
import re
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
WINDOWS = os.name == "nt"


def main():
    if os.environ.get("GITHUB_ACTIONS") != "true":
        raise SystemExit("This online installation check runs only in managed GitHub Actions.")
    with tempfile.TemporaryDirectory(prefix="mafsil-online-bootstrap-") as folder:
        root = pathlib.Path(folder)
        home = root / "home space [literal]"
        home.mkdir()
        local = home / "local"
        local.mkdir()
        temporary = root / "temporary"
        temporary.mkdir()
        workspace = root / "existing workspace مرحبا"
        workspace.mkdir()
        (workspace / "keep.txt").write_text("operator data", encoding="utf-8")
        env = os.environ | {"HOME": str(home), "USERPROFILE": str(home), "LOCALAPPDATA": str(local), "TEMP": str(temporary), "TMP": str(temporary), "TMPDIR": str(temporary)}
        destination = local / "Programs/Mafsil" if WINDOWS else home / ".local/share/mafsil"
        if WINDOWS:
            bootstrap = ["pwsh", "-NoProfile", "-File", str(ROOT / "install.ps1"), "-Destination", str(destination)]
            management = ["pwsh", "-NoProfile", "-File", str(destination / "install.ps1"), "-Destination", str(destination)]
        else:
            bootstrap = ["sh", str(ROOT / "install.sh"), "--destination", str(destination)]
            management = ["sh", str(destination / "install.sh"), "--destination", str(destination)]
        def run(command, **kwargs):
            return subprocess.run(command, env=env, cwd=root, check=True, timeout=240, **kwargs)
        run(bootstrap)
        try:
            readme = (ROOT / "README.md").read_text(encoding="utf-8")
            setup = readme.split("### Choose an existing workspace\n", 1)[1].split("### Connect an MCP client\n", 1)[0]
            language = "powershell" if WINDOWS else "bash"
            code = re.search(r"```" + language + r"\n(.*?)\n```", setup, re.S)[1]
            script = root / ("readme-setup.ps1" if WINDOWS else "readme-setup.sh")
            # The fixture sends UTF-8 bytes to redirected stdin; an interactive
            # Windows console normally receives Read-Host text as Unicode.
            prefix = "[Console]::InputEncoding = [Text.UTF8Encoding]::new($false)\n" if WINDOWS else ""
            script.write_text(prefix + code, encoding="utf-8", newline="\n")
            command = ["pwsh", "-NoProfile", "-File", str(script)] if WINDOWS else ["bash", str(script)]
            run(command, input=str(workspace) + "\n", text=True, encoding="utf-8")
            config = destination / "config.json"
            before = config.read_bytes()
            assert json.loads(before)["workspace"] == str(workspace)
            scratch = destination / "scratch"
            scratch.mkdir()
            (scratch / "keep.txt").write_text("keep scratch", encoding="utf-8")
            run(bootstrap)
            assert config.read_bytes() == before
            assert (scratch / "keep.txt").read_text() == "keep scratch"
            assert (workspace / "keep.txt").read_text() == "operator data"
        finally:
            run(management + (["-Action", "Uninstall"] if WINDOWS else ["--action", "uninstall"]))
        assert config.read_bytes() == before
        assert not (destination / ("mafsil.exe" if WINDOWS else "mafsil")).exists()
        print("Public bootstrap, README workspace setup, repeat update and uninstall passed in a temporary home.")


if __name__ == "__main__":
    main()
