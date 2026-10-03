#!/usr/bin/env python3
"""Inspect package checksums and exercise the native release binary over stdio."""
import argparse
import hashlib
import json
import os
import pathlib
import platform
import queue
import struct
import subprocess
import tempfile
import threading

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--bundle", type=pathlib.Path, required=True)
    args = parser.parse_args()
    bundle = args.bundle.resolve()
    manifest = {}
    for line in (bundle / "SHA256SUMS").read_text(encoding="ascii").splitlines():
        digest, name = line.split("  ")
        assert pathlib.Path(name).name == name and name not in manifest
        with (bundle / name).open("rb") as stream:
            assert hashlib.file_digest(stream, "sha256").hexdigest() == digest, name
        manifest[name] = digest
    assert set(manifest) == {p.name for p in bundle.iterdir()} - {"SHA256SUMS"}
    for name in manifest:
        if not name.startswith("mafsil_v"):
            continue
        data = (bundle / name).read_bytes()[:4096]
        if "windows" in name:
            assert data[:2] == b"MZ"
            pe = struct.unpack_from("<I", data, 0x3C)[0]
            assert data[pe:pe+4] == b"PE\0\0" and struct.unpack_from("<H", data, pe+4)[0] == 0x8664
        else:
            assert data[:5] == b"\x7fELF\x02"
            assert struct.unpack_from("<H", data, 18)[0] == (183 if "arm64" in name else 62)
    system = "windows" if os.name == "nt" else "linux"
    arch = "arm64" if platform.machine().lower() in ("arm64", "aarch64") else "amd64"
    binary = bundle / f"mafsil_v0.1.0_{system}_{arch}{'.exe' if os.name == 'nt' else ''}"
    if os.name != "nt":
        # Raw release downloads do not carry executable filesystem mode bits.
        binary.chmod(binary.stat().st_mode | 0o111)
    flags = subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0
    assert subprocess.check_output([str(binary), "version"], text=True, encoding="utf-8", creationflags=flags).startswith("Mafsil 0.1.0 (")
    with tempfile.TemporaryDirectory(prefix="mafsil-distribution-test-") as root:
        root = pathlib.Path(root)
        work = root / "workspace"; work.mkdir()
        (work / "example.txt").write_bytes("Mafsil مرحبا\r\n".encode())
        config = root / "config.json"
        subprocess.run([str(binary), "init", "--workspace", str(work), "--output", str(config)], check=True, capture_output=True, creationflags=flags)
        doctor = subprocess.check_output([str(binary), "doctor", "--config", str(config)], text=True, encoding="utf-8", creationflags=flags)
        assert json.loads(doctor)["ok"]
        for modern in (False, True):
            proc = subprocess.Popen([str(binary), "serve", "--config", str(config)], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, creationflags=flags)
            messages = queue.Queue()
            def receive():
                for line in proc.stdout:
                    messages.put(json.loads(line))
            reader = threading.Thread(target=receive, daemon=True); reader.start()
            def request(identifier, method, params):
                if modern:
                    params = params | {"_meta": {"io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": {}}}
                proc.stdin.write(json.dumps({"jsonrpc": "2.0", "id": identifier, "method": method, "params": params}).encode() + b"\n")
                proc.stdin.flush()
                result = messages.get(timeout=10)
                assert result["id"] == identifier, result
                assert "error" not in result, result
                return result["result"]
            try:
                if modern:
                    assert request(1, "server/discover", {})["resultType"] == "complete"
                else:
                    assert request(1, "initialize", {"protocolVersion": "2025-11-25", "capabilities": {}, "clientInfo": {"name": "distribution-test", "version": "1"}})["serverInfo"]["name"] == "Mafsil"
                    proc.stdin.write(b'{"jsonrpc":"2.0","method":"notifications/initialized"}\n'); proc.stdin.flush()
                names = {t["name"] for t in request(2, "tools/list", {})["tools"]}
                assert names == {"server_info", "read_file", "list_directory"}
                result = request(3, "tools/call", {"name": "read_file", "arguments": {"path": "example.txt"}})
                assert not result.get("isError")
                assert json.loads(result["content"][0]["text"])["text"] == "Mafsil مرحبا\r\n"
                denied = request(4, "tools/call", {"name": "read_file", "arguments": {"path": "../config.json"}})
                assert denied["isError"]
            finally:
                proc.stdin.close()
                try:
                    proc.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    proc.kill(); proc.wait(); raise AssertionError("server did not exit on stdio EOF")
                reader.join(timeout=2)
                error = proc.stderr.read()
                assert proc.returncode == 0, error
                assert not reader.is_alive()
    print("Distribution headers, checksums, CLI and both MCP protocol eras passed.")

if __name__ == "__main__":
    main()
