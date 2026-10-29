#!/usr/bin/env python3
"""Join the checked ADR/test map with go test -json runtime evidence.
Usage: python3 scripts/verify-scenarios.py GO_TEST_JSON OUT_JSON
Native-only tests are reported as not run, never as passed on this host.
"""
from pathlib import Path
import json, re, sys
repo = Path(__file__).resolve().parents[1]
registry = json.loads((repo / 'docs/adr/scenario-tests.json').read_text())
results = {}
for line in Path(sys.argv[1]).read_text().splitlines():
    event = json.loads(line)
    if event.get('Test') and event['Action'] in ('pass', 'fail', 'skip'):
        key = event['Package'].removeprefix('skill-audit/') + ':' + event['Test']
        results[key] = event['Action']
native_windows = set()
for source in repo.glob('**/*_windows_test.go'):
    if 'vendor' in source.parts:
        continue
    package = str(source.parent.relative_to(repo))
    native_windows.update(package + ':' + name for name in re.findall(r'^func (Test\w+)\(', source.read_text(), re.M))
rows = {}
for scenario, refs in registry['scenarios'].items():
    tests = {}
    for ref in refs:
        status = results.get(ref)
        if status is None:
            base = ref.split(':', 1)[0] + ':' + ref.split(':', 1)[1].split('/', 1)[0]
            assert base in native_windows, ('mapped test did not run', ref)
            status = 'not_run_native_windows'
        assert status != 'fail', (scenario, ref)
        tests[ref] = status
    rows[scenario] = {'tests': tests}
    if scenario in registry.get('requirements', {}):
        rows[scenario]['limit'] = registry['requirements'][scenario]
out = {'accepted_scenarios': len(rows), 'runtime_evidence': rows}
Path(sys.argv[2]).write_text(json.dumps(out, indent=2) + '\n')
not_run = [scenario for scenario, row in rows.items() if all(status.startswith('not_run') for status in row['tests'].values())]
print(json.dumps({'mapped_scenarios': len(rows), 'native_windows_not_run': not_run, 'limitations': registry['requirements']}, indent=2))
