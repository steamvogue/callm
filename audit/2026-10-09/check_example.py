#!/usr/bin/env python3
"""Validate the report's shell example using a fake callm, without network calls."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

report = Path(__file__).with_name('REPORT.md').read_text()
script = Path(sys.argv[1]).read_text() if len(sys.argv) > 1 else re.search(r'```bash\n(#!/usr/bin/env bash.*?)\n```', report, re.S).group(1)

def envelope(content, reason='stop', refusal=None):
    return {'choices': [{'finish_reason': reason,
                        'message': {'content': content, 'refusal': refusal}}]}

cases = [
    ('valid', envelope('{"summary":"valid result"}'), True, 0),
    ('literal-tags-preserved', envelope('{"summary":"<think>literal</think>"}'), True, 0),
    ('truncated', envelope('{"summary":"partial"}', 'length'), False, 0),
    ('refusal', envelope(None, refusal='synthetic refusal'), False, 0),
    ('missing-choices', {}, False, 0),
    ('invalid-json-content', envelope('{bad'), False, 0),
    ('wrong-type', envelope('[]'), False, 0),
    ('missing-summary', envelope('{}'), False, 0),
    ('empty-summary', envelope('{"summary":""}'), False, 0),
    ('provider-failure', envelope('{"summary":"unused"}'), False, 1),
]
observations = []
with tempfile.TemporaryDirectory(prefix='callm-example-') as tmp:
    root = Path(tmp)
    fake = root / 'callm'
    fake.write_text('#!/usr/bin/python3\nimport os,sys\nprint(os.environ["AUDIT_REPLY"])\nsys.exit(int(os.environ["AUDIT_EXIT"]))\n')
    fake.chmod(0o700)
    example = root / 'example.sh'
    example.write_text(script + '\n')
    subprocess.run(['shellcheck', str(example)], check=True)
    (root / 'input.txt').write_text('sample document\n')
    for name, response, success, code in cases:
        dest = root / 'summary.json'
        dest.write_text('original artifact\n')
        env = os.environ | {'PATH': str(root) + os.pathsep + os.environ['PATH'],
                            'AUDIT_REPLY': json.dumps(response), 'AUDIT_EXIT': str(code)}
        r = subprocess.run(['bash', str(example)], cwd=root, env=env, capture_output=True, text=True)
        assert (r.returncode == 0) == success, (name, r.stderr)
        if success:
            assert json.loads(dest.read_text()) == json.loads(response['choices'][0]['message']['content'])
        else:
            assert dest.read_text() == 'original artifact\n', name
        assert not list(root.glob('summary.json.tmp.*')), name
        observations.append({'case': name, 'passed': True, 'exit': r.returncode})
print(json.dumps(observations, indent=2))
