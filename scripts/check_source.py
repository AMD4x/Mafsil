#!/usr/bin/env python3
"""Check source hygiene without storing secret values or machine identifiers."""
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]

def main():
    result = subprocess.run(["git", "ls-files", "-co", "--exclude-standard", "-z"], cwd=ROOT, capture_output=True, check=True)
    files = sorted(set(result.stdout.decode("utf-8").split("\0")) - {""})
    checks = [
        ("private key", re.compile(rb"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----")),
        ("credential", re.compile(rb"(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,}|sk-(?:proj-)?[A-Za-z0-9_-]{25,})")),
        ("machine path", re.compile(rb"(?:[A-Z]:\\Users\\[^\\\s]+\\|/home/[a-zA-Z0-9_.-]+/)", re.I)),
        ("runtime state", re.compile(rb"(?i)(?:tunnel_)[0-9a-f]{32}")),
    ]
    problems = []
    for name in files:
        path = ROOT / name
        if path.suffix.lower() in (".exe", ".test", ".syso", ".dpapi", ".log") or path.name.startswith(".env"):
            problems.append((name, "unpublishable generated or secret file"))
        if path.suffix.lower() in (".png", ".ico"):
            continue
        if path.is_symlink():
            problems.append((name, "unexpected source symlink"))
            continue
        data = path.read_bytes()
        for label, pattern in checks:
            if pattern.search(data):
                problems.append((name, label))
    for name, label in problems:
        print(f"{name}: {label}", file=sys.stderr)
    if problems:
        raise SystemExit(1)
    print(f"Source hygiene passed ({len(files)} files; secret values are never printed).")

if __name__ == "__main__":
    main()
