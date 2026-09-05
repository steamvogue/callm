#!/usr/bin/env python3
"""Local-only behavioral audit. Builds the CLI and uses dummy keys and loopback HTTP.

Run: python3 audit/probe_cli.py > audit/probe-results.json
Regression assertions cover the repaired findings. No third-party API requests are made.
"""
import base64
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = Path(__file__).resolve().parents[1]
captures = []
observations = []


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_GET(self):
        self.respond()

    def do_POST(self):
        self.respond()

    def respond(self):
        body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        req = json.loads(body) if body else {}
        captures.append({'path': self.path, 'body': req,
                         'authorization': self.headers.get('Authorization'),
                         'x-api-key': self.headers.get('x-api-key')})
        model = req.get('model', '')
        if '/fail/' in self.path:
            self.send_response(429)
            self.end_headers()
            self.wfile.write(b'{"error":{"message":"rate limited"}}')
            return
        self.send_response(200)
        self.send_header('Content-Type', 'text/event-stream' if req.get('stream') else 'application/json')
        self.end_headers()
        if model == 'stall':
            self.wfile.flush()
            time.sleep(2)
            return
        if self.path.endswith('/models'):
            result = {'data': [{'id': 'test-model'}]}
        elif self.path.endswith('/messages'):
            result = {'id': 'ant-test', 'content': [{'type': 'text', 'text': 'answer'}],
                      'usage': {'input_tokens': 3, 'output_tokens': 2}, 'stop_reason': 'end_turn'}
        else:
            result = {'id': 'test', 'model': model, 'provider_extension': 'keep-me',
                      'choices': [{'message': {'role': 'assistant',
                                  'content': '<think>private</think>answer' if model == 'inline' else 'answer',
                                  'thought': 'thought-only' if model == 'thought' else '',
                                  'reasoning': 'reason' if model == 'reason' else ''}, 'finish_reason': 'stop'}],
                      'usage': {'prompt_tokens': 3, 'completion_tokens': 2, 'total_tokens': 5,
                                'cost': '0.125' if model == 'string-cost' else 0.125}}
        if model == 'body-error':
            result = {'error': {'message': 'provider failure'}}
        if req.get('stream'):
            if self.path.endswith('/messages'):
                data = 'data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"answer"}}\n\ndata: {"type":"message_stop"}\n\n'
            elif model == 'json-stream':
                data = json.dumps(result)
            elif model == 'malformed':
                data = 'data: {broken-json}\n\ndata: [DONE]\n\n'
            else:
                delta = {'content': '<think>private</think>answer' if model == 'inline' else 'answer'}
                if model == 'reason':
                    delta['reasoning'] = 'reason'
                payload = json.dumps({'choices': [{'delta': delta}]})
                prefix = 'data:' if model == 'no-space' else 'data: '
                data = prefix + payload + '\n\n'
                if model != 'truncated':
                    if req.get('stream_options', {}).get('include_usage'):
                        data += 'data: ' + json.dumps({'choices': [], 'usage': result['usage']}) + '\n\n'
                    data += 'data: [DONE]\n\n'
            self.wfile.write(data.encode())
        else:
            self.wfile.write(json.dumps(result).encode())


def record(name, result):
    observations.append({'name': name, **result})


with tempfile.TemporaryDirectory(prefix='callm-audit-') as temporary:
    temp = Path(temporary)
    binary = temp / 'callm'
    subprocess.run(['go', 'build', '-o', str(binary), './cmd/callm'], cwd=ROOT, check=True)
    server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    base = f'http://127.0.0.1:{server.server_port}'
    env = {k: v for k, v in os.environ.items() if not k.endswith(('_API_KEY', '_MODEL', '_BASE_URL'))}
    env.update({'CALLM_BASE_URL': base, 'CALLM_API_KEY': 'audit-dummy', 'AUDIT_KEY': 'explicit-env-dummy'})

    def run(name, args, *, extra=None, stdin=None):
        before = len(captures)
        result = subprocess.run([str(binary), *args], input=stdin, capture_output=True,
                                text=True, env=env | (extra or {}), cwd=temp, timeout=4)
        record(name, {'exit': result.returncode, 'stdout': result.stdout, 'stderr': result.stderr,
                      'requests': captures[before:]})
        return result

    # Exposed chat knobs, including aliases and explicit zero values.
    run('combined-generic-knobs', ['--no-stream', '-m', 'custom', '-s', 'system text', '-t', '0',
                                  '-n', '321', '--top-p', '0',
                                  '--effort', 'medium', '--json-object',
                                  '--stats', 'prompt'])
    for flag in ['-m', '--model']:
        run('alias ' + flag, ['--no-stream', flag, 'custom-model', 'prompt'])
    for flag in ['-k', '--key', '--api-key']:
        run('alias ' + flag, ['--no-stream', flag, 'explicit-dummy', 'prompt'])
    for flag in ['--key-env', '--api-key-env']:
        run('alias ' + flag, ['--no-stream', flag, 'AUDIT_KEY', 'prompt'])
    for flag in ['-s', '--system']:
        run('alias ' + flag, ['--no-stream', flag, 'system text', 'prompt'])
    for flag in ['-t', '--temp', '--temperature']:
        run('alias ' + flag, ['--no-stream', flag, '0.7', 'prompt'])
    for flag in ['-n', '--max-tokens']:
        run('alias ' + flag, ['--no-stream', flag, '128', 'prompt'])
    for flag in ['--effort', '--reasoning-effort']:
        run('alias ' + flag, ['--no-stream', flag, 'low', 'prompt'])
    for flag in ['--api', '--base-url']:
        run('alias ' + flag, ['--no-stream', flag, base + '/custom', 'prompt'])
    for flag in ['--st', '--or', '--ds', '--ant', '--anthropic', '--claude', '--ms', '--moonshot',
                 '--kimi', '--zai', '--glm', '--qw', '--qwen', '--oa', '--openai', '--groq', '--ollama']:
        run('preset ' + flag, [flag, '--no-stream', 'prompt'])
    context = temp / 'context.txt'
    context.write_text('context-data')
    pic = temp / 'pixel.png'
    pic.write_bytes(base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/l9sAAAAASUVORK5CYII='))
    run('files-and-images', ['--no-stream', '-f', str(context), '--file', str(context),
                            '--image', str(pic), '--image', 'https://example.com/image.png', 'prompt'])
    run('ready-stdin', ['--no-stream', 'prompt'], stdin='input-context')
    run('default-streaming-redirected-output', ['prompt'])
    for flags in [['--stream'], ['--no-stream'], ['--stream=false'], ['--no-stream', '--stream'],
                  ['--reasoning'], ['--no-reasoning'], ['--only-reasoning'],
                  ['--only-reasoning', '--no-reasoning']]:
        run('output ' + ' '.join(flags), [*flags, '-m', 'reason', 'prompt'])
    for model in ['inline', 'thought']:
        for flags in [['--no-reasoning'], ['--only-reasoning'], ['--reasoning']]:
            run('nonstream ' + model + ' ' + flags[0], ['--no-stream', '-m', model, *flags, 'prompt'])
    run('json-is-lossy-and-skips-stats', ['--json', '--stats', 'prompt'])
    run('string-cost', ['--no-stream', '--stats', '-m', 'string-cost', 'prompt'])
    run('stream-stats-with-opt-in-usage-server', ['--stream', '--stats', 'prompt'])
    for model in ['malformed', 'truncated', 'no-space', 'json-stream']:
        run('stream ' + model, ['--stream', '-m', model, 'prompt'])
    run('nonstream-body-error', ['--no-stream', '-m', 'body-error', 'prompt'])
    run('raw-http-error', ['raw', '--api', base + '/fail', '/endpoint', '{}'])
    run('raw-unknown-flag', ['raw', '--bogus', '/endpoint', '{}'])
    run('models-command', ['models', '--oa', 'test'])
    run('documented-preset-before-models', ['--oa', 'models', 'test'])
    run('info-command', ['info', 'test-model'])
    run('info-global-url-precedence', ['info', '--oa', 'test-model'],
        extra={'OPENAI_BASE_URL': base + '/provider', 'CALLM_BASE_URL': base + '/global'})
    for option, value in [('--max-tokens', '123junk'), ('--temp', '0.7junk'), ('--top-p', '2'),
                          ('--max-tokens', '-1'), ('--effort', 'banana')]:
        run('invalid ' + option + '=' + value, ['--no-stream', option, value, 'prompt'])
    run('dual-o3-token-limits', ['--no-stream', '-m', 'o3-mini', '-n', '10', '--max-completion-tokens', '20', 'prompt'])
    run('gateway-effort-and-budget', ['--api', base + '/openrouter.ai', '--no-stream', '--effort', 'high',
                                     '--thinking-budget', '1500', 'prompt'])
    run('anthropic-rejects-unsupported-json', ['--ant', '--api', base + '/api.anthropic.com', '--no-stream',
                                   '--top-p', '0.8', '--json-object', '--image', str(pic), 'prompt'])
    run('anthropic-exceeds-explicit-cap', ['--ant', '--api', base + '/api.anthropic.com', '--no-stream',
                                          '-n', '100', '--thinking-budget', '1024', 'prompt'])
    for args in [['--version'], ['-v'], ['version'], ['--help'], ['chat', '--help']]:
        run('local-command ' + ' '.join(args), args)

    run('anthropic-image-and-top-p', ['--ant', '--no-stream', '--top-p', '0.8', '--image', str(pic), 'prompt'])
    run('gateway-budget', ['--or', '--no-stream', '--thinking-budget', '1500', 'prompt'])
    run('max-completion-tokens-only', ['--no-stream', '--max-completion-tokens', '654', 'prompt'])
    run('stdin-timeout', ['--stdin-timeout', '0.01', 'prompt'], stdin='ready')
    run('provider-conflict', ['--or', '--st', 'prompt'])
    run('stdin-ignore', ['--no-stdin', 'prompt'], stdin='should-be-ignored')

    # Delay the producer without relying on a shell pipeline scheduling race.
    p = subprocess.Popen([str(binary), '--no-stream', 'summarize'], stdin=subprocess.PIPE,
                         stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env=env, cwd=temp)
    before = len(captures)
    time.sleep(0.15)
    try:
        out, err = p.communicate('delayed-important-context', timeout=2)
    except BrokenPipeError:
        out, err = p.communicate(timeout=2)
    record('delayed-stdin', {'exit': p.returncode, 'stdout': out, 'stderr': err, 'requests': captures[before:]})

    # Verify cancellation and bounded behavior with a deliberately stalled server.
    for name, args in [('stream-stall', ['--stream', '-m', 'stall', 'prompt']),
                       ('anthropic-nonstream-stall', ['--api', base + '/api.anthropic.com', '--no-stream', '-m', 'stall', 'prompt'])]:
        p = subprocess.Popen([str(binary), *args], stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE, text=True, env=env, cwd=temp)
        time.sleep(0.25)
        alive = p.poll() is None
        p.send_signal(signal.SIGTERM)
        out, err = p.communicate(timeout=2)
        record(name, {'alive_after_250ms': alive, 'exit_after_sigterm': p.returncode, 'stdout': out, 'stderr': err})

    read_fd, write_fd = os.pipe()
    os.write(write_fd, b'partial-input')
    p = subprocess.Popen([str(binary), 'prompt'], stdin=read_fd, stdout=subprocess.PIPE,
                         stderr=subprocess.PIPE, text=True, env=env, cwd=temp)
    os.close(read_fd)
    time.sleep(0.15)
    p.send_signal(signal.SIGTERM)
    try:
        out, err = p.communicate(timeout=0.25)
        blocked = False
    except subprocess.TimeoutExpired:
        blocked = True
        p.kill()
        out, err = p.communicate()
    os.close(write_fd)
    record('stdin-open-pipe-cancellation', {'still_blocked_after_sigterm': blocked, 'stdout': out, 'stderr': err})
    server.shutdown()

print(json.dumps(observations, indent=2))

# Assert repaired behavior; any regression makes this script fail.
by_name = {item['name']: item for item in observations}
def body(name):
    return by_name[name]['requests'][0]['body']
def expect(condition, message):
    if not condition:
        raise AssertionError(message)
for name in ['stream malformed', 'stream truncated', 'stream json-stream', 'nonstream-body-error',
             'raw-http-error', 'raw-unknown-flag', 'anthropic-rejects-unsupported-json',
             'anthropic-exceeds-explicit-cap', 'dual-o3-token-limits', 'gateway-effort-and-budget',
             'provider-conflict', 'output --no-stream --stream', 'output --only-reasoning --no-reasoning']:
    expect(by_name[name]['exit'] != 0, name + ' should fail')
for item in observations:
    if item['name'].startswith('invalid '):
        expect(item['exit'] != 0 and not item['requests'], item['name'] + ' sent invalid values')
expect(by_name['stream no-space']['stdout'].strip() == 'answer', 'SSE without a space failed')
expect('delayed-important-context' in body('delayed-stdin')['messages'][0]['content'], 'delayed input lost')
expect(not by_name['stdin-open-pipe-cancellation']['still_blocked_after_sigterm'], 'stdin cancellation blocked')
expect('private' not in by_name['nonstream inline --no-reasoning']['stdout'], 'hidden reasoning leaked')
expect('private' in by_name['nonstream inline --only-reasoning']['stderr'], 'inline reasoning lost')
expect('thought-only' in by_name['nonstream thought --only-reasoning']['stderr'], 'thought fallback lost')
expect('provider_extension' in by_name['json-is-lossy-and-skips-stats']['stdout'], 'raw JSON lost fields')
expect('[stats:' in by_name['json-is-lossy-and-skips-stats']['stderr'], 'JSON stats lost')
expect('$0.125000' in by_name['string-cost']['stderr'], 'string cost lost')
expect('5 tokens' in by_name['stream-stats-with-opt-in-usage-server']['stderr'], 'stream usage missing')
expect(by_name['documented-preset-before-models']['requests'][0]['path'] == '/models', 'global command dispatch failed')
expect(not body('output --stream=false').get('stream'), 'false stream flag ignored')
expect(not body('default-streaming-redirected-output').get('stream'), 'redirected stdout should default to non-streaming')
expect(body('anthropic-image-and-top-p')['top_p'] == 0.8, 'Anthropic top-p dropped')
expect(body('anthropic-image-and-top-p')['messages'][0]['content'][1]['type'] == 'image', 'Anthropic image schema wrong')
expect(body('gateway-budget')['reasoning']['max_tokens'] == 1500, 'proxy gateway budget lost')
expect(body('max-completion-tokens-only')['max_completion_tokens'] == 654, 'completion token cap lost')
expect(body('stdin-ignore')['messages'][0]['content'] == 'prompt', 'no-stdin ignored')
