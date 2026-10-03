#!/usr/bin/env python3
"""Verify and assemble native CI candidates. Never publish from this script."""
import argparse
import hashlib
import json
import pathlib
import re
import shutil

TARGETS = ("windows_amd64", "linux_amd64", "linux_arm64")
COMMON = {"LICENSE", "THIRD_PARTY_NOTICES.md", "install.ps1", "install.sh", "DEPENDENCIES.json"}

def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=pathlib.Path, required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    if not re.fullmatch(r"[a-f0-9]{40}", args.commit) or not re.fullmatch(r"v\d+\.\d+\.\d+", args.version):
        parser.error("exact commit SHA and version are required")
    args.output.mkdir(parents=True, exist_ok=True)
    if any(args.output.iterdir()):
        parser.error("output must be empty")
    collected = {}
    for target in TARGETS:
        folder = args.input / ("candidate-" + target)
        binary = f"mafsil_{args.version}_{target}" + (".exe" if target.startswith("windows") else "")
        info = "BUILD-INFO_" + target + ".json"
        expected = COMMON | {binary, info}
        manifest = {}
        for line in (folder / "SHA256SUMS").read_text(encoding="ascii").splitlines():
            match = re.fullmatch(r"([a-f0-9]{64})  ([A-Za-z0-9_.-]+)", line)
            if not match or match[2] in manifest:
                raise SystemExit("Malformed or duplicate manifest record")
            manifest[match[2]] = match[1]
        if set(manifest) != expected or {p.name for p in folder.iterdir()} != expected | {"SHA256SUMS"}:
            raise SystemExit(f"Unexpected candidate contents: {target}")
        for name, checksum in manifest.items():
            path = folder / name
            if path.is_symlink() or not path.is_file() or digest(path) != checksum:
                raise SystemExit(f"Candidate checksum/type mismatch: {name}")
            if name in collected and collected[name][0] != checksum:
                raise SystemExit(f"Cross-platform common file differs: {name}")
            collected[name] = (checksum, path)
        build = json.loads((folder / info).read_text(encoding="utf-8"))
        if build["commit"] != args.commit or build["version"] != args.version or build["target"] != target.replace("_", "/"):
            raise SystemExit(f"Candidate provenance differs from approved commit/version: {target}")
    for name, (_, source) in collected.items():
        shutil.copy2(source, args.output / name)
    (args.output / "SHA256SUMS").write_text("".join(f"{collected[name][0]}  {name}\n" for name in sorted(collected)), encoding="ascii", newline="\n")
    print(f"Verified {len(collected)} assets from commit {args.commit}")

if __name__ == "__main__":
    main()
