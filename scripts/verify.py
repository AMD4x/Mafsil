#!/usr/bin/env python3
"""Portable checks; callers select their own Go caches and temporary directory."""
import argparse
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]

def run(*args):
    subprocess.run(args, check=True, cwd=ROOT)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--security", action="store_true")
    parser.add_argument("--race", action="store_true")
    args = parser.parse_args()
    files = [str(p.relative_to(ROOT)) for parent in (ROOT / "cmd", ROOT / "internal") for p in parent.rglob("*.go")]
    unformatted = subprocess.check_output(["gofmt", "-l", *files], cwd=ROOT, text=True, encoding="utf-8").strip()
    if unformatted:
        raise SystemExit("Run gofmt on:\n" + unformatted)
    run("go", "mod", "verify")
    run("go", "vet", "./...")
    run("go", "test", "-shuffle=on", "-timeout=180s", "./...")
    run(sys.executable, "scripts/licenses.py", "--check")
    run(sys.executable, "scripts/check_source.py")
    run(sys.executable, "tests/assembly_test.py", "-v")
    run(sys.executable, "tests/bootstrap_test.py", "-v")
    if args.race:
        run("go", "test", "-race", "-shuffle=on", "-timeout=180s", "./...")
    if args.security:
        run("go", "run", "honnef.co/go/tools/cmd/staticcheck@v0.8.1", "./...")
        run("go", "run", "golang.org/x/vuln/cmd/govulncheck@v1.8.0", "./...")

if __name__ == "__main__":
    main()
