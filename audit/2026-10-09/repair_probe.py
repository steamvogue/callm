#!/usr/bin/env python3
"""After-repair regressions: synthetic responses, dummy credentials, loopback only.

Build first: GOCACHE=/tmp/callm-audit-go-cache go build -o /tmp/callm-repair-20261009 ./cmd/callm
Run: python3 audit/2026-10-09/repair_probe.py > audit/2026-10-09/repair-probes.json
Assertions describe repaired behavior; the original probe/results remain unchanged.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

requests = []


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_GET(self):
        requests.append({'method': 'GET', 'path': self.path})
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps({'data': [{'id': 'claude-sonnet-5-5',
            'max_input_tokens': 1000000, 'max_tokens': 128000,
            'capabilities': {'thinking': {'supported': True}}}]}).encode())

    def do_POST(self):
        req = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        requests.append({'path': self.path, 'body': req})
        model = req['model']
        stream = req.get('stream', False)
        self.send_response(200)
        self.send_header('Content-Type', 'text/event-stream' if stream else 'application/json')
        self.end_headers()
        usage = {'input_tokens': 10, 'output_tokens': 5,
                 'cache_read_input_tokens': 100, 'cache_creation_input_tokens': 20}
        if self.path.endswith('/messages'):
            if stream:
                events = [
                    {'type': 'message_start', 'message': {'usage': usage}},
                    {'type': 'content_block_delta', 'delta': {'type': 'text_delta', 'text': 'partial'}},
                    {'type': 'message_delta', 'delta': {'stop_reason': 'max_tokens'}, 'usage': {'output_tokens': 5}},
                    {'type': 'message_stop'},
                ]
                data = ''.join('data: ' + json.dumps(e) + '\n\n' for e in events)
            else:
                data = json.dumps({'model': model, 'content': [{'type': 'text', 'text': 'partial'}],
                                   'stop_reason': 'max_tokens', 'usage': usage})
        else:
            content = 'OK'
            reason = 'stop'
            message = {'role': 'assistant', 'content': content}
            if model in ('truncated', 'stream-truncated'):
                message['content'], reason = 'partial', 'length'
            elif model == 'literal-think':
                message['content'] = 'Example: <think>literal text</think> end'
            elif model == 'json-think':
                message['content'] = '{"example":"<think>literal text</think>"}'
            elif model == 'refusal':
                message = {'role': 'assistant', 'content': None, 'refusal': 'Synthetic refusal'}
            elif model == 'filtered':
                message['content'], reason = None, 'content_filter'
            result = {'model': model, 'choices': [{'message': message, 'finish_reason': reason}]}
            if model == 'empty':
                result = {}
            if stream:
                chunk = {'choices': [{'delta': {'content': message.get('content')}, 'finish_reason': reason}]}
                data = 'data: ' + json.dumps(chunk) + '\n\ndata: [DONE]\n\n'
                if model == 'done-only':
                    data = 'data: [DONE]\n\n'
            else:
                data = json.dumps(result)
        self.wfile.write(data.encode())


with tempfile.TemporaryDirectory(prefix='callm-evaluation-') as directory:
    root = Path(directory)
    for name in ('bin', 'work', 'home'):
        (root / name).mkdir()
    binary = root / 'bin' / 'callm'
    shutil.copy2('/tmp/callm-repair-20261009', binary)
    server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    base = f'http://127.0.0.1:{server.server_port}/v1'
    env = {'HOME': str(root / 'home'), 'CALLM_LOAD_DOTENV': '0'}
    results = []

    def run(name, options, *, full=False, extra_env=None):
        before = len(requests)
        args = [str(binary), '--api', base, '--api-key', 'audit-dummy', *options]
        output = open('/dev/full', 'w') if full else subprocess.PIPE
        try:
            result = subprocess.run(args, cwd=root / 'work', env=env | (extra_env or {}),
                stdin=subprocess.DEVNULL, stdout=output, stderr=subprocess.PIPE,
                text=True, timeout=5)
        finally:
            if full:
                output.close()
        results.append({'case': name, 'exit': result.returncode, 'stdout': result.stdout,
                        'stderr': result.stderr, 'requests': requests[before:]})
        return results[-1]

    for model in ('truncated', 'empty', 'refusal', 'filtered', 'literal-think', 'json-think'):
        result = run(model, ['--oa', '--no-stream', '--no-stdin', '-m', model, 'prompt'])
        assert (result['exit'] == 0) == (model in ('literal-think', 'json-think')), result
        if model == 'literal-think':
            assert result['stdout'] == 'Example: <think>literal text</think> end\n', result
        if model == 'json-think':
            assert json.loads(result['stdout']) == {'example': '<think>literal text</think>'}, result
    for model in ('stream-truncated', 'done-only'):
        result = run(model, ['--oa', '--stream', '--no-stdin', '-m', model, 'prompt'])
        assert result['exit'] != 0, result
    for flag in ('--no-stream', '--stream'):
        result = run('anthropic-cache-and-stop-' + flag, ['--ant', flag, '--no-stdin',
                     '--stats', '-m', 'cache', 'prompt'])
        assert result['exit'] != 0 and '135 tokens (130 in / 5 out)' in result['stderr'], result
    result = run('stdout-write-failure', ['--oa', '--no-stream', '--no-stdin', 'prompt'], full=True)
    assert result['exit'] != 0 and 'no space' in result['stderr'].lower(), result
    result = run('gpt6-token-cap', ['--oa', '--no-stream', '--no-stdin', '-m', 'gpt-6.1-sol',
                 '--max-tokens', '200', 'prompt'])
    assert result['requests'][0]['body']['max_completion_tokens'] == 200 and 'max_tokens' not in result['requests'][0]['body'], result
    result = run('luna-none-supported', ['--oa', '--no-stream', '--no-stdin', '-m', 'gpt-6-luna',
                 '--effort', 'none', 'prompt'])
    assert result['exit'] == 0 and result['requests'][0]['body']['reasoning_effort'] == 'none', result
    result = run('sonnet55-adaptive-thinking', ['--ant', '--no-stream', '--no-stdin', '-m',
                 'claude-sonnet-5-5', '--effort', 'high', '--json', 'prompt'])
    assert result['requests'][0]['body']['thinking'] == {'type': 'adaptive'} and result['requests'][0]['body']['output_config'] == {'effort': 'high'}, result
    for format_name in ('zed', 'kilo', 'continue'):
        result = run('anthropic-export-' + format_name, ['--ant', 'models', '--format', format_name])
        assert result['exit'] != 0 and not result['requests'] and not result['stdout'], result
    # No key override here: observe default-provider selection with only OpenAI configured.
    result = subprocess.run([str(binary), '--no-stdin', 'prompt'], cwd=root / 'work',
        env=env | {'OPENAI_API_KEY': 'audit-dummy'}, stdin=subprocess.DEVNULL,
        capture_output=True, text=True, timeout=5)
    assert result.returncode != 0 and 'POOLSIDE_API_KEY' in result.stderr and 'OPENAI_API_KEY: --oa' in result.stderr and 'audit-dummy' not in result.stderr, result
    results.append({'case': 'only-openai-key', 'exit': result.returncode,
                    'stdout': result.stdout, 'stderr': result.stderr})
    for name, options in (
        ('json-truncated-diagnostic', ['--oa', '--json', '-m', 'truncated', 'prompt']),
        ('json-truncated-strict', ['--oa', '--json', '--strict', '-m', 'truncated', 'prompt']),
        ('astra-none-rejected', ['--oa', '-m', 'gpt-6-astra', '--effort', 'none', 'prompt']),
        ('sol-sampling-rejected', ['--oa', '-m', 'gpt-6.1-sol', '--temperature', '0.2', 'prompt']),
        ('luna-none-sampling', ['--oa', '-m', 'gpt-6-luna', '--effort', 'none', '--temperature', '0.2', 'prompt']),
        ('sonnet-budget-rejected', ['--ant', '-m', 'claude-sonnet-5-5', '--thinking-budget', '1024', 'prompt']),
        ('sonnet-sampling-rejected', ['--ant', '-m', 'claude-sonnet-5-5', '--temperature', '0.2', 'prompt']),
    ):
        result = run(name, ['--no-stdin', *options])
        valid = name in ('json-truncated-diagnostic', 'luna-none-sampling')
        assert (result['exit'] == 0) == valid, result
        if name.endswith('-rejected'):
            assert not result['requests'], result
        if name == 'json-truncated-strict':
            assert not result['stdout'], result
    result = run('anthropic-json-catalog', ['--ant', 'models', '--format', 'json'])
    assert result['exit'] == 0 and json.loads(result['stdout'])[0]['capabilities']['thinking']['supported'], result
    for options, name in (
        (['--oa', '--stream', 'prompt'], 'stream'),
        (['--oa', '--json', 'prompt'], 'json'),
        (['--oa', 'raw', '/chat/completions', '{"model":"OK","messages":[]}'], 'raw'),
        (['--oa', 'info', 'claude-sonnet-5-5'], 'info'),
        (['--oa', 'models'], 'table'),
    ):
        result = run('full-destination-' + name, options, full=True)
        assert result['exit'] != 0, result
    server.shutdown()
    print(json.dumps(results, indent=2))
