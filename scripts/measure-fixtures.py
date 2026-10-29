#!/usr/bin/env python3
"""Small macOS-only wait4 resource observation; never scans installed skills.
Usage: python3 scripts/measure-fixtures.py BASELINE_BINARY REWRITE_BINARY OUT.json
The numbers describe this warm-cache fixture, not general endpoint performance.
"""
import json
from pathlib import Path
import platform
import os
import shutil
import subprocess
import sys
import tempfile
import time

def main():
    if platform.system() != "Darwin":
        raise SystemExit("This measurement wrapper has only been validated on macOS; native alternatives remain pending.")
    root = Path(tempfile.mkdtemp(prefix="skill-audit-resources-")).resolve()
    try:
        selected = root / "selected"
        selected.mkdir()
        content = b"fixture bytes are data only\n" * 1260
        for n in range(100):
            p = selected / ("p%03d" % n)
            p.mkdir()
            (p / "SKILL.md").write_text("---\nname: fixture-%d\n---\n# fixture\n" % n)
            for f in range(8):
                (p / ("data%d.txt" % f)).write_bytes(content)
        result = {"host": platform.platform(), "fixture": {"packages": 100, "files": 900,
                  "content_bytes": sum(p.stat().st_size for p in selected.rglob("*") if p.is_file())},
                  "samples": {}}
        for label, arg in zip(("baseline", "rewrite"), sys.argv[1:3]):
            binary = Path(arg).resolve()
            samples = []
            for repeat in range(3):
                inv = root / (label + "-inventory.json")
                for phase, args in (("inventory", ["inventory", "--root", str(selected), "--out", str(inv)]),
                                    ("audit", ["audit", "--inventory", str(inv), "--out", str(root / (label + "-report.json"))])):
                    start = time.perf_counter()
                    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
                        p = subprocess.Popen([str(binary), *args], stdout=stdout, stderr=stderr)
                        _, status, usage = os.wait4(p.pid, 0)
                        p.returncode = os.waitstatus_to_exitcode(status)
                        stderr.seek(0)
                        error = stderr.read().decode()
                    wall = (time.perf_counter() - start) * 1000
                    assert p.returncode == 0, (label, phase, p.returncode, error)
                    samples.append({"repeat": repeat + 1, "phase": phase, "exit": p.returncode,
                                    "wall_ms": round(wall, 3), "max_rss_bytes": usage.ru_maxrss})
            result["samples"][label] = samples
        output = Path(sys.argv[3]).resolve()
        output.write_text(json.dumps(result, indent=2) + "\n")
        print(json.dumps(result, indent=2))
    finally:
        shutil.rmtree(root)
if __name__ == "__main__":
    main()
