#!/usr/bin/env python3
"""Fourth-review CLI fixtures. No network, device scan or skill execution.
Usage: python3 scripts/review4-smoke.py BINARY GO_FAKE_SCANNER BEFORE_BINARY OUT.json
"""
from pathlib import Path
import base64, copy, json, shutil, subprocess, sys, tempfile
binary, helper, before_binary, output = (Path(s).resolve() for s in sys.argv[1:5])
repo = Path(__file__).resolve().parents[1]
root = Path(tempfile.mkdtemp(prefix="skill-audit-review4-")).resolve()
commands = []
def run(name, args, expected=0, before=False):
    exe = before_binary if before else binary
    p = subprocess.run([str(exe), *map(str, args)], capture_output=True, text=True)
    commands.append({"name": name, "before": before, "exit": p.returncode})
    assert p.returncode == expected, (name, p.returncode, p.stdout, p.stderr)
    return json.loads(p.stdout) if p.stdout.lstrip().startswith("{") else None
try:
    selected = root / "selected"
    package = selected / "p"
    package.mkdir(parents=True)
    (package / "SKILL.md").write_text("# fixture\n")
    inv = root / "inventory.json"
    run("inventory", ["inventory", "--root", selected, "--out", inv])
    inventory = json.loads(inv.read_text())
    for p in inventory["packages"]:
        p["digest"] = p["digest"].upper()
        for f in p["files"]:
            f["sha256"] = f["sha256"].upper()
    inv.write_text(json.dumps(inventory))
    original_inventory = inv.read_bytes()
    args = ["audit", "--inventory", inv, "--json"]
    old = run("uppercase_inventory", args, 2, before=True)
    assert not old["complete"]
    new = run("uppercase_inventory", args)
    assert new["complete"] and inv.read_bytes() == original_inventory
    manifest = json.loads((repo / "internal/audit/catalog/manifest.json").read_text())
    ref = manifest["skills"][0]
    files = repo / "internal/audit/catalog/skills" / ref["id"]
    data = {f["path"]: base64.b64encode((files / f["path"]).read_bytes()).decode() for f in ref["files"]}
    store = {"schema": "skill-audit/catalog-store/v2", "entries": [{"reference": ref, "data": data}], "attribution": {}, "diagnostics": []}
    catalog = root / "catalog.json"
    catalog.write_text(json.dumps(store))
    lower = run("lowercase_catalog", [*args, "--catalog", catalog])
    upper = copy.deepcopy(store)
    for entry in upper["entries"]:
        entry["reference"]["digest"] = entry["reference"]["digest"].upper()
        for f in entry["reference"]["files"]:
            f["sha256"] = f["sha256"].upper()
    catalog.write_text(json.dumps(upper))
    original_catalog = catalog.read_bytes()
    run("uppercase_catalog", [*args, "--catalog", catalog], 2, before=True)
    canonical = run("uppercase_catalog", [*args, "--catalog", catalog])
    assert canonical["catalog_version"] == lower["catalog_version"] and catalog.read_bytes() == original_catalog
    scanner = root / "scan-partial-exit2.exe"
    shutil.copyfile(helper, scanner)
    scanner.chmod(0o700)
    scan_args = [*args, "--xray", scanner]
    old = run("partial_scanner", scan_args, 2, before=True)
    new = run("partial_scanner", scan_args, 2)
    old_eval = old["skills"][0]["evaluations"][-1]
    evaluation = new["skills"][0]["evaluations"][-1]
    assert "coverage incomplete" not in old_eval["message"]
    assert "coverage incomplete" in evaluation["message"] and "scanner exit 2" in evaluation["message"]
    search_args = ["search", "--registry-url", "https://registry.invalid/v1", "--query", "x" * 181, "--json"]
    old = run("rejected_registry_query", search_args, 2, before=True)
    new = run("rejected_registry_query", search_args, 2)
    assert old["inputs"][0]["source"] == "github-code-search" and new["inputs"][0]["source"] == "skills-sh-v1"
    actual = root / ".claude/skills"
    actual.mkdir(parents=True)
    (actual / "p").mkdir()
    (actual / "p/SKILL.md").write_text("# fixture\n")
    alias = root / ".Claude/skills"
    alias_checked = alias.exists() and actual.samefile(alias)
    if alias_checked:
        roots = ["inventory", "--root", actual, "--root", alias, "--json"]
        old = run("root_alias", roots, before=True)
        new = run("root_alias", roots)
        assert not any(d["code"] == "root_delegated" for d in old.get("diagnostics", []))
        assert any(d["code"] == "root_delegated" for d in new["diagnostics"]) and new["complete"]
    state = root / "state.json"
    run("stored_time", [*args, "--state", state, "--record", "baseline"])
    history = json.loads(state.read_text())
    assert "observed_at" in history["observations"][0] and "created_at" not in history["observations"][0]["report"]
    output.write_text(json.dumps({"checks": {"uppercase_inventory": True, "uppercase_catalog_and_identity": True, "input_bytes_unchanged": True, "scanner_causes_preserved": True, "selected_provider": True, "root_alias": alias_checked, "single_stored_time": True}, "commands": commands}, indent=2) + "\n")
    print("fourth-review CLI checks passed:", len(commands), "commands; alias tested:", alias_checked)
finally:
    shutil.rmtree(root)
