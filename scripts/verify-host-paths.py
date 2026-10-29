#!/usr/bin/env python3
"""Reproduce the Unix filename baseline bug and verify recovery with temp fixtures.
Usage: python3 scripts/verify-host-paths.py BEFORE AFTER GO_FAKE_SCANNER OUT_JSON
No network, installed skills or skill-script execution.
"""
from pathlib import Path
import json, shutil, subprocess, sys, tempfile
before, after, helper, output = (Path(s).resolve() for s in sys.argv[1:5])
if sys.platform == 'win32':
    raise SystemExit('This reproduction uses colon/backslash Unix filenames; run native Windows Go path tests instead.')
root = Path(tempfile.mkdtemp(prefix='skill-audit-host-paths-')).resolve()
commands = []
def run(binary, name, args, expected):
    p = subprocess.run([str(binary), *map(str, args)], capture_output=True, text=True)
    commands.append({'binary': 'before' if binary == before else 'after', 'name': name, 'exit': p.returncode})
    assert p.returncode == expected, (name, p.returncode, p.stdout, p.stderr)
    return json.loads(p.stdout) if '--json' in args else None
try:
    package = root / 'package'
    package.mkdir()
    (package / 'SKILL.md').write_text('# fixture\n')
    for name in ('a:b.md', 'literal\\name.md'):
        (package / name).write_text('fixture bytes\n')
    inventory, state = root / 'inventory.json', root / 'state.json'
    run(before, 'inventory', ['inventory', '--root', package, '--out', inventory], 0)
    args = ['audit', '--inventory', inventory, '--state', state, '--json']
    saved = run(before, 'record_unreadable_baseline', [*args, '--record', 'baseline'], 0)
    assert saved['recording']['status'] == 'recorded'
    broken = run(before, 'read_unreadable_baseline', args, 2)
    assert any(d['code'] == 'state_invalid' for d in broken['diagnostics'])
    original = state.read_bytes()
    recovered = run(after, 'recover_existing_baseline', args, 0)
    assert recovered['lineage'][0]['status'] == 'unchanged' and not recovered['lineage'][0]['changes']
    assert state.read_bytes() == original
    run(after, 'append_unchanged_snapshot', [*args, '--record', 'snapshot'], 0)
    (package / 'a:b.md').write_text('changed fixture bytes\n')
    run(after, 'refresh_inventory', ['inventory', '--root', package, '--out', inventory], 0)
    changed = run(after, 'append_changed_snapshot', [*args, '--record', 'snapshot'], 0)
    assert changed['lineage'][0]['changes'] == [{'path': 'a:b.md', 'kind': 'modified'}]
    reread = run(after, 'read_three_observations', args, 0)
    assert reread['lineage'][0]['status'] == 'changed'
    assert len(json.loads(state.read_text())['observations']) == 3
    scanner = root / 'fake scanner.exe'
    shutil.copyfile(helper, scanner)
    scanner.chmod(0o700)
    scan_args = ['audit', '--inventory', inventory, '--xray', scanner, '--json']
    rejected = run(before, 'old_staging_refusal', scan_args, 2)
    assert rejected['skills'][0]['evaluations'][-1]['status'] == 'failed'
    scanned = run(after, 'new_staging_success', scan_args, 0)
    assert scanned['complete'] and scanned['skills'][0]['evaluations'][-1]['status'] == 'completed_without_findings'
    output.write_text(json.dumps({'before_revision': '70316378c3407b62839d9c13c2146ef3818662dd', 'checks': {'old_bug_reproduced': True, 'existing_baseline_recovered_without_write': True, 'unchanged_and_changed_snapshots_readable': True, 'host_names_staged': True}, 'commands': commands}, indent=2) + '\n')
    print('host filename CLI checks passed:', len(commands), 'commands')
finally:
    shutil.rmtree(root)
