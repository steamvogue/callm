#!/usr/bin/env python3
"""Test live-script assertions with a fake CLI; never invoke a provider."""
import json
import os
from pathlib import Path
import subprocess
import tempfile

script = Path(__file__).resolve().parents[2] / 'scripts/test_live.sh'
answer = '9.9 is larger'
fixtures = [
    ('native-omitted-thinking', 'ant', {'content':[{'type':'thinking','thinking':''},{'type':'text','text':answer}], 'usage':{'output_tokens':5}}, True),
    ('native-text-only-with-notice', 'ant', {'content':[{'type':'text','text':answer}], 'usage':{'output_tokens':5}}, False),
    ('native-zero-usage', 'ant', {'content':[{'type':'thinking','thinking':''},{'type':'text','text':answer}], 'usage':{'output_tokens':0}}, False),
    ('compatible-reasoning-field', 'ds', {'choices':[{'message':{'content':answer,'reasoning_content':'summary'}}], 'usage':{'completion_tokens':5}}, True),
    ('compatible-reasoning-usage', 'ds', {'choices':[{'message':{'content':answer}}], 'usage':{'completion_tokens':5,'completion_tokens_details':{'reasoning_tokens':2}}}, True),
    ('compatible-notice-only', 'ds', {'choices':[{'message':{'content':answer}}], 'usage':{'completion_tokens':5}}, False),
]
results = []
with tempfile.TemporaryDirectory(prefix='callm-live-script-') as directory:
    root=Path(directory)
    fake=root/'fake-callm'
    fake.write_text('''#!/usr/bin/python3
import os,sys
print('callm: synthetic dotenv notice', file=sys.stderr)
if '--json' in sys.argv:
    print(os.environ['FIXTURE'])
else:
    print(sys.argv[-1].split('Reply exactly: ',1)[1])
''')
    fake.chmod(0o700)
    for name,preset,fixture,success in fixtures:
        env={'PATH':os.environ['PATH'],'CALLM_TEST_BIN':str(fake),'FIXTURE':json.dumps(fixture)}
        env['ANTHROPIC_API_KEY' if preset=='ant' else 'DEEPSEEK_API_KEY']='test-dummy'
        result=subprocess.run(['bash',str(script)],cwd=root,env=env,capture_output=True,text=True,timeout=5)
        assert (result.returncode==0)==success,(name,result.stdout,result.stderr)
        results.append({'case':name,'passed':True,'exit':result.returncode,'stdout':result.stdout,'stderr':result.stderr})
print(json.dumps(results,indent=2))
