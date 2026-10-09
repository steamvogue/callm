#!/usr/bin/env python3
"""Test the real callm and example runner against localhost fixtures only."""
import http.server
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import threading

root = pathlib.Path(__file__).resolve().parents[1]
binary = pathlib.Path(os.environ.get("CALLM_TEST_BIN", str(root / "bin/callm"))).resolve()
calls = []
fail_check = False


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        calls.append(body)
        prompt = body["messages"][-1]["content"]
        answer = {"invoice": "INV-17", "currency": "EUR", "cents": 4250} if "Extract the attached" in prompt else ({"valid": False} if fail_check else {"valid": True})
        response = {"model": "mock/free", "choices": [{"finish_reason": "stop", "message": {"content": json.dumps(answer)}}], "usage": {"prompt_tokens": 12, "completion_tokens": 20, "total_tokens": 32, "cost": 0}}
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(response).encode())


server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
observations = []
try:
    with tempfile.TemporaryDirectory(prefix="callm-harness-mocks-") as tmp:
        work = pathlib.Path(tmp)
        source = work / "source"
        shutil.copytree(root / "examples/pipeline", source)
        manifest = json.loads((source / "manifest.json").read_text())
        manifest["api"] = f"http://127.0.0.1:{server.server_port}"
        (source / "manifest.json").write_text(json.dumps(manifest))
        output = work / "out"
        env = {"PATH": os.environ["PATH"], "HOME": str(work), "OPENROUTER_API_KEY": "test-dummy", "CALLM_LOAD_DOTENV": "0"}
        command = ["python3", str(root / "examples/mini-harness.py"), str(source / "manifest.json"), "--output-dir", str(output), "--callm", str(binary)]

        def check(name, options, expected_calls, success=True):
            before = len(calls)
            result = subprocess.run(command + options, env=env, capture_output=True, text=True, timeout=10)
            assert (result.returncode == 0) == success, (name, result.stdout, result.stderr)
            assert len(calls) - before == expected_calls, (name, calls)
            assert not (output / ".lock").exists()
            observations.append({"case": name, "passed": True, "mock_calls": expected_calls, "exit_code": result.returncode})

        check("two-step-success", [], 2)
        assert json.loads((output / "extract.json").read_text())["cents"] == 4250
        check("resume-skips-unchanged", ["--resume"], 0)
        (output / "extract.json").write_text('{"tampered":true}')
        check("tampered-output-recomputed", ["--resume"], 1)
        (source / "extract.txt").write_text((source / "extract.txt").read_text() + "\nBe precise.")
        check("changed-input-recomputed", ["--resume"], 1)
        previous = (output / "check.json").read_bytes()
        fail_check = True
        (source / "check.txt").write_text((source / "check.txt").read_text() + "\nCheck carefully.")
        check("failed-schema-preserves-artifact", ["--resume"], 1, False)
        assert (output / "check.json").read_bytes() == previous
        journal = json.loads((output / "journal.json").read_text())
        assert journal["steps"]["check"]["status"] == "error"
        fail_check = False
        check("resume-retries-failed-step-on-new-invocation", ["--resume"], 1)
        manifest["steps"][0]["inputs"] = ["step:check"]
        (source / "manifest.json").write_text(json.dumps(manifest))
        check("forward-reference-rejected", ["--resume"], 0, False)
finally:
    server.shutdown()
    server.server_close()
print(json.dumps(observations, indent=2))
