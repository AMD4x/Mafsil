#!/usr/bin/env python3
"""Build source-first release candidates into an explicitly selected directory.

No publishing, installing, git mutation, or network calls except Go module fetches.
"""
import argparse
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
TARGETS = ("windows/amd64", "linux/amd64", "linux/arm64")

def run(*args, **kwargs):
    subprocess.run(args, check=True, cwd=kwargs.pop("cwd", ROOT), **kwargs)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", required=True, type=pathlib.Path)
    parser.add_argument("--target", choices=TARGETS, action="append")
    args = parser.parse_args()
    if not re.fullmatch(r"v\d+\.\d+\.\d+", args.version):
        parser.error("version must be vMAJOR.MINOR.PATCH")
    output = args.output.resolve()
    if output == ROOT or ROOT in output.parents:
        parser.error("output must be outside the source tree")
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        parser.error("output directory must be empty")
    version = args.version[1:]
    # Working trees are legitimate local candidates, never advertised as a commit.
    commit = "local"
    try:
        if not subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT, text=True).strip():
            commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    except subprocess.CalledProcessError:
        pass
    inventory = {"version": args.version, "commit": commit, "go": subprocess.check_output(["go", "version"], text=True).strip(), "targets": {}}
    with tempfile.TemporaryDirectory(prefix="mafsil-package-") as scratch:
        stage = pathlib.Path(scratch) / "source"
        stage.mkdir()
        for name in ("go.mod", "go.sum"):
            shutil.copy2(ROOT / name, stage / name)
        for name in ("cmd", "internal"):
            shutil.copytree(ROOT / name, stage / name)
        for target in args.target or TARGETS:
            system, arch = target.split("/")
            resource = stage / "cmd/mafsil/resource_windows_amd64.syso"
            if system == "windows":
                numbers = dict(zip(("Major", "Minor", "Patch"), map(int, version.split(".")))) | {"Build": 0}
                resource_config = {"FixedFileInfo": {"FileVersion": numbers, "ProductVersion": numbers, "FileFlagsMask": "3f", "FileFlags": "00", "FileOS": "040004", "FileType": "01"}, "StringFileInfo": {"CompanyName": "Ahmed Mustafa", "FileDescription": "Mafsil MCP workspace tools", "FileVersion": version, "ProductVersion": version, "ProductName": "Mafsil", "OriginalFilename": "mafsil.exe", "LegalCopyright": "Copyright (c) 2026 AMD4x"}, "VarFileInfo": {"Translation": {"LangID": "0409", "CharsetID": "04B0"}}}
                spec = pathlib.Path(scratch) / "versioninfo.json"
                spec.write_text(json.dumps(resource_config), encoding="utf-8")
                run("go", "run", "github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.7.0", "-64", "-icon", str(ROOT / "assets/brand/mafsil.ico"), "-o", str(resource), str(spec))
            suffix = ".exe" if system == "windows" else ""
            name = f"mafsil_{args.version}_{system}_{arch}{suffix}"
            env = os.environ | {"GOOS": system, "GOARCH": arch, "CGO_ENABLED": "0"}
            run("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", f"-s -w -buildid= -X main.version={version} -X main.commit={commit}", "-o", str(output / name), "./cmd/mafsil", cwd=stage, env=env)
            inventory["targets"][target] = subprocess.check_output(["go", "version", "-m", str(output / name)], text=True).replace(str(output / name), name)
            if resource.exists():
                resource.unlink()
    for name in ("LICENSE", "THIRD_PARTY_NOTICES.md"):
        shutil.copy2(ROOT / name, output / name)
    for name in ("install.ps1", "install.sh"):
        shutil.copy2(ROOT / "scripts" / name, output / name)
    for target, information in inventory["targets"].items():
        entry = {"version": args.version, "commit": commit, "go": inventory["go"], "target": target, "modules": information}
        (output / ("BUILD-INFO_" + target.replace("/", "_") + ".json")).write_text(json.dumps(entry, indent=2) + "\n", encoding="utf-8", newline="\n")
    # A compact, inspectable dependency inventory supplements go version -m.
    modules = subprocess.check_output(["go", "list", "-m", "-json", "all"], cwd=ROOT, text=True, encoding="utf-8")
    decoder, remaining, items = json.JSONDecoder(), modules, []
    while remaining.strip():
        remaining = remaining.lstrip()
        module, consumed = decoder.raw_decode(remaining)
        items.append({k: module[k] for k in ("Path", "Version", "Sum", "GoModSum") if k in module})
        remaining = remaining[consumed:]
    (output / "DEPENDENCIES.json").write_text(json.dumps(items, indent=2) + "\n", encoding="utf-8", newline="\n")
    hashes = []
    for path in sorted(output.iterdir()):
        with path.open('rb') as stream:
            hashes.append(f"{hashlib.file_digest(stream, 'sha256').hexdigest()}  {path.name}")
    (output / "SHA256SUMS").write_text("\n".join(hashes) + "\n", encoding="ascii", newline="\n")
    print(f"Packaged {len(inventory['targets'])} target(s) in {output}")

if __name__ == "__main__":
    main()
