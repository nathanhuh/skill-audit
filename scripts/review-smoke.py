#!/usr/bin/env python3
"""Fixture-only checks for the 7a84ccc review. No skill scripts are executed.
Usage: python3 scripts/review-smoke.py BINARY GO_FAKE_SCANNER OUTPUT_DIRECTORY [--large]
--large exercises the production 64 MiB inventory bound with temporary metadata.
"""
from pathlib import Path
import json, os, shutil, subprocess, sys, tempfile
binary, helper, out = map(lambda s: Path(s).resolve(), sys.argv[1:4])
out.mkdir(parents=True, exist_ok=True)
root = Path(tempfile.mkdtemp(prefix="skill-audit-review-", dir="/private/tmp" if sys.platform=="darwin" else None)).resolve()
commands=[]
def run(name,args,want,env=None):
    with (out/(name+".stderr.txt")).open("w") as stderr:
        p=subprocess.run([str(binary),*map(str,args)],stdout=subprocess.DEVNULL,stderr=stderr,env=env)
    commands.append({"name":name,"args":[str(a).replace(str(root),"<fixture>").replace(str(out),"<output>") for a in args],"exit":p.returncode})
    assert p.returncode==want,(name,p.returncode,(out/(name+".stderr.txt")).read_text())
def inventory(name,selected,extra=()):
    path=out/(name+".json");run(name,["inventory","--root",selected,"--out",path,*extra],0);return path
try:
    pkg=root/"selected/p";pkg.mkdir(parents=True);(pkg/"SKILL.md").write_text("# fixture\n")
    inv=inventory("initial",pkg)
    catalog=out/"input-catalog.json"
    catalog.write_text(json.dumps({"schema":"skill-audit/catalog-store/v2","entries":[],"attribution":{},"diagnostics":[{"code":"audit_interrupted","path":"untrusted","message":"fixture input","incomplete":True}]}))
    report=out/"e8-report.json"
    run("imported_interruption",["audit","--inventory",inv,"--catalog",catalog,"--state",root/"recorded-state.json","--record","baseline","--out",report],2)
    assert json.loads(report.read_text())["recording"]["status"]=="recorded"
    metadata=root/"metadata/.git/p";metadata.mkdir(parents=True);(metadata/"SKILL.md").write_text("# fixture")
    rejected=out/"metadata.json"
    run("explicit_metadata",["inventory","--root",metadata.parent,"--out",rejected],2)
    v=json.loads(rejected.read_text());assert not v["complete"] and any(d["code"]=="excluded_root" for d in v["diagnostics"])
    actual=root/"actual";(actual/"x").mkdir(parents=True);(actual/"x/SKILL.md").write_text("# fixture")
    if sys.platform=="darwin":
        alias=Path("/tmp")/root.name/"actual/x"
    else:
        (root/"alias").symlink_to(actual,target_is_directory=True);alias=root/"alias/x"
    alias_inv=out/"aliases.json"
    run("rejected_alias",["inventory","--root",alias,"--root",actual,"--out",alias_inv],2)
    v=json.loads(alias_inv.read_text());assert len(v["packages"])==1 and not v["complete"]
    second=root/"selected/q";second.mkdir();(second/"SKILL.md").write_text("# fixture")
    both=inventory("both",pkg.parent)
    scanner=out/"scan-flood.exe";shutil.copyfile(helper,scanner);scanner.chmod(0o700)
    if Path(str(scanner)+".log").exists():Path(str(scanner)+".log").unlink()
    flood_report=out/"flood.json"
    run("console_limit",["audit","--inventory",both,"--xray",scanner,"--out",flood_report],2)
    v=json.loads(flood_report.read_text());assert all(any(e["scanner"]=="skill-xray" and e["status"]=="partial" and "console output exceeded" in e["message"] for e in s["evaluations"]) for s in v["skills"])
    assert Path(str(scanner)+".log").read_text().splitlines().count("scan")==2
    state_inside=pkg.parent/"state.json"
    mismatch=out/"state-mismatch.json"
    run("state_mismatch",["audit","--inventory",both,"--state",state_inside,"--record","baseline","--out",mismatch],3)
    assert not state_inside.exists() and any(d["code"]=="state_exclusion_unconfirmed" for d in json.loads(mismatch.read_text())["diagnostics"])
    temp_base=pkg/"temp";temp_base.mkdir();env=os.environ.copy();env.update({k:str(temp_base) for k in ("TMPDIR","TMP","TEMP")})
    temp_report=out/"temp.json"
    run("unsafe_temp",["audit","--inventory",both,"--xray",scanner,"--out",temp_report],2,env)
    assert list(temp_base.iterdir())==[]
    v=json.loads(temp_report.read_text());assert all(any(e["scanner"]=="skill-xray" and "staging refused" in e.get("message","") for e in s["evaluations"]) for s in v["skills"])
    gaps=root/"gaps";(gaps/"p").mkdir(parents=True);(gaps/"p/SKILL.md").write_text("# fixture")
    for i in range(3000):(gaps/("link-%04d"%i)).symlink_to(pkg,target_is_directory=True)
    gap_inv=out/"gaps.json"
    run("gap_budget",["inventory","--root",gaps,"--out",gap_inv],2)
    v=json.loads(gap_inv.read_text());assert v["diagnostics_truncated"] and v["omitted_diagnostics"]["symlink_skipped"]>0 and len(v["packages"])==1
    large_result=None
    if "--large" in sys.argv:
        large=root/"large";large.mkdir();body="---\nname: "+"n"*70000+"\n---\n# fixture\n"
        for i in range(1000):
            d=large/("p%04d"%i);d.mkdir();(d/"SKILL.md").write_text(body)
        large_inv=out/"large.json"
        run("production_output_budget",["inventory","--root",large,"--out",large_inv],2)
        v=json.loads(large_inv.read_text());assert not v["complete"] and v["budget_stop"] and 0<len(v["packages"])<1000 and all(p["complete"] and len(p["files"])==1 for p in v["packages"])
        assert large_inv.stat().st_size<=64<<20
        large_result={"bytes":large_inv.stat().st_size,"retained_whole_packages":len(v["packages"]),"candidate_packages":1000};del v
    result={"reviewed_revision":"7a84ccc0a4d2c910606e73239c3b343db96ed98c","commands":commands,"production_inventory_budget":large_result,"status":"passed"}
    (out/"results.json").write_text(json.dumps(result,indent=2)+"\n")
    print(json.dumps({"commands":len(commands),"status":"passed","production_inventory_budget":large_result},indent=2))
finally:
    shutil.rmtree(root)
