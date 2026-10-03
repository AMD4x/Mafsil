#!/usr/bin/env python3
"""Collect complete notices for modules linked into Mafsil and the Go runtime."""
import argparse
import json
import pathlib
import re
import subprocess

ROOT = pathlib.Path(__file__).resolve().parents[1]

def render_notices(sections):
    rendered = ["# Third-party notices\n\nMafsil's original code and brand assets are MIT licensed.\nThird-party code retains its own licenses. No bundled font files."]
    for heading, source, content in sections:
        # Keep upstream legal text literal; Markdown must not reinterpret it.
        fence = "`" * max(3, max((len(run) + 1 for run in re.findall(r"`+", content)), default=3))
        link = f"\n\n[Upstream source]({source})" if source else ""
        rendered.append(f"## {heading}{link}\n\n{fence}text\n{content.rstrip()}\n{fence}")
    return "\n\n".join(rendered) + "\n"

def objects(text):
    decoder = json.JSONDecoder()
    while text.strip():
        text = text.lstrip()
        obj, offset = decoder.raw_decode(text)
        yield obj
        text = text[offset:]

def generate():
    raw = subprocess.check_output(["go", "list", "-deps", "-json", "./cmd/mafsil"], cwd=ROOT, text=True, encoding="utf-8")
    modules = {}
    for package in objects(raw):
        module = package.get("Module", {})
        if module and not module.get("Main"):
            modules[module["Path"]] = module
    goroot = pathlib.Path(subprocess.check_output(["go", "env", "GOROOT"], text=True).strip())
    sections = [("Go standard library/runtime", "", (goroot / "LICENSE").read_text(encoding="utf-8"))]
    for name, module in sorted(modules.items()):
        directory = pathlib.Path(module["Dir"])
        files = sorted(p for p in directory.iterdir() if p.is_file() and (p.name.upper().startswith(("LICENSE", "COPYING")) or p.name.upper() in ("NOTICE", "NOTICE.TXT")))
        if not files:
            raise RuntimeError(f"No license found for {name}")
        sections.append((f"{name} {module['Version']}", f"https://{name}", "\n\n".join(p.read_text(encoding="utf-8") for p in files)))
    return render_notices(sections)

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    generated = generate()
    target = ROOT / "THIRD_PARTY_NOTICES.md"
    if args.check:
        if target.read_text(encoding="utf-8") != generated:
            raise SystemExit("THIRD_PARTY_NOTICES.md is stale; run python scripts/licenses.py")
    else:
        target.write_text(generated, encoding="utf-8", newline="\n")
