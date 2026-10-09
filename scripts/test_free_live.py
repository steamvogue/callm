#!/usr/bin/env python3
"""Exercise free-live guards with a fake CLI; no provider calls."""
import json
import os
import pathlib
import subprocess
import tempfile

script = pathlib.Path(__file__).with_name("test_live.sh").resolve()
model = "google/gemma-4-26b-a4b-it:free"
cases = [("free", model, {"prompt": "0", "completion": "0"}, True),
         ("paid-id", "paid/model", {"prompt": "0", "completion": "0"}, False),
         ("nonzero-price", model, {"prompt": "0.1", "completion": "0"}, False),
         ("missing-price", model, {"completion": "0"}, False),
         ("malformed-price", model, {"prompt": "unknown", "completion": "0"}, False),
         ("extra-nonzero-price", model, {"prompt": "0", "completion": "0", "request": "1"}, False)]
with tempfile.TemporaryDirectory(prefix="callm-free-live-mocks-") as temp:
    root = pathlib.Path(temp)
    fake = root / "callm"
    fake.write_text('''#!/usr/bin/python3
import json,os,sys
if 'models' in sys.argv:
    print(os.environ['CATALOG'])
else:
    with open(os.environ['COUNTER'],'a') as f: f.write('call\\n')
    print(json.dumps({'choices':[{'finish_reason':'stop','message':{'content':sys.argv[-1].split('Reply exactly: ')[1]}}],'usage':{'completion_tokens':4,'cost':0}}))
''')
    fake.chmod(0o700)
    observations = []
    for name, selected, prices, valid in cases:
        counter = root / "calls"
        counter.write_text("")
        env = {"PATH": os.environ["PATH"], "OPENROUTER_API_KEY": "test-dummy", "CALLM_TEST_BIN": str(fake),
               "CALLM_TEST_FREE_MODEL": selected, "CATALOG": json.dumps([{"id": model, "pricing": prices}]), "COUNTER": str(counter)}
        proc = subprocess.run(["bash", str(script)], env=env, capture_output=True, text=True, timeout=5)
        assert (proc.returncode == 0) == valid, (name, proc.stdout, proc.stderr)
        calls = len(counter.read_text().splitlines())
        assert calls == (2 if valid else 0), (name, calls)
        observations.append({"case": name, "passed": True, "generation_calls": calls, "exit_code": proc.returncode})
print(json.dumps(observations, indent=2))
