#!/usr/bin/env python3
"""Sequential, bounded callm example: schema checks, atomic artifacts, hash resume.

This optional runner requires Python 3 on Linux/macOS. callm itself remains a
standalone binary. Manifest paths are relative to the manifest's directory.
"""
import argparse
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

MAX_INPUT = 64 << 20


def digest(data):
    return hashlib.sha256(data).hexdigest()


def read_file(path, limit=MAX_INPUT):
    if not path.is_file() or path.stat().st_size > limit:
        raise ValueError(f"Expected regular file of at most {limit} bytes: {path}")
    with path.open("rb") as f:
        data = f.read(limit + 1)
    if len(data) > limit:
        raise ValueError(f"File exceeds {limit} bytes: {path}")
    return data


def atomic_write(path, data):
    fd, tmp = tempfile.mkstemp(prefix=".callm-", dir=path.parent)
    try:
        with os.fdopen(fd, "wb") as f:
            f.write(data)
            f.flush()
            os.fsync(f.fileno())
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)


def json_bytes(value):
    return (json.dumps(value, indent=2, ensure_ascii=False) + "\n").encode()


def load_manifest(path):
    manifest = json.loads(read_file(path, 1 << 20))
    allowed = {"version", "provider", "model", "api", "key_env", "max_tokens", "timeout_seconds", "steps"}
    if not isinstance(manifest, dict) or set(manifest) - allowed:
        raise ValueError("Manifest must be an object with documented fields only")
    if type(manifest.get("version")) is not int or manifest["version"] != 1 or manifest.get("provider") not in {"or", "ollama", "oa", "ant", "st", "ds", "pool", "kimi", "orca"}:
        raise ValueError("Manifest requires version 1 and an explicit supported provider")
    if not isinstance(manifest.get("model"), str) or not manifest["model"]:
        raise ValueError("Manifest requires an explicit model ID")
    for name, default, maximum in [("max_tokens", 2048, 16384), ("timeout_seconds", 60, 600)]:
        value = manifest.get(name, default)
        if type(value) is not int or not 1 <= value <= maximum:
            raise ValueError(f"{name} must be an integer from 1 to {maximum}")
        manifest[name] = value
    if not isinstance(manifest.get("api"), str) or not manifest["api"].startswith(("http://", "https://")):
        raise ValueError("Manifest requires an explicit http(s) API URL to avoid endpoint environment overrides")
    if "key_env" in manifest and (not isinstance(manifest["key_env"], str) or not re.fullmatch(r"[A-Za-z_][A-Za-z_0-9]*", manifest["key_env"])):
        raise ValueError("key_env must name an environment variable; never store a key value")
    steps = manifest.get("steps")
    if not isinstance(steps, list) or not 1 <= len(steps) <= 16:
        raise ValueError("Manifest requires 1–16 sequential steps")
    seen = set()
    for step in steps:
        if not isinstance(step, dict) or set(step) - {"id", "prompt_file", "system_file", "schema", "inputs"}:
            raise ValueError("Invalid step fields")
        sid = step.get("id")
        if not isinstance(sid, str) or not re.fullmatch(r"[A-Za-z0-9_-]{1,64}", sid) or sid in seen:
            raise ValueError("Step IDs must be unique simple names")
        for name in ("prompt_file", "schema"):
            if not isinstance(step.get(name), str) or not step[name]:
                raise ValueError(f"Step {sid} requires {name}")
        if "system_file" in step and not isinstance(step["system_file"], str):
            raise ValueError("system_file must be a path")
        inputs = step.get("inputs", [])
        if not isinstance(inputs, list) or len(inputs) > 32 or any(not isinstance(i, str) or not i for i in inputs):
            raise ValueError("inputs must be a list of at most 32 paths or step:ID references")
        for value in inputs:
            if value.startswith("step:") and value[5:] not in seen:
                raise ValueError("step:ID inputs must refer to earlier steps")
        seen.add(sid)
    return manifest


def run(args):
    source = pathlib.Path(args.manifest).resolve()
    manifest = load_manifest(source)
    binary = pathlib.Path(shutil.which(args.callm) or args.callm).resolve()
    binary_hash = digest(read_file(binary))
    out = pathlib.Path(args.output_dir).resolve()
    out.mkdir(mode=0o700, parents=True, exist_ok=True)
    # Refuse simultaneous writers. A lock left after a forced termination must
    # be removed by the operator after checking that no runner is still active.
    lock = out / ".lock"
    lock_fd = os.open(lock, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    os.close(lock_fd)
    try:
        journal_path = out / "journal.json"
        journal = {"version": 1, "steps": {}}
        if args.resume and journal_path.exists():
            journal = json.loads(read_file(journal_path, 1 << 20))
            if journal.get("version") != 1 or not isinstance(journal.get("steps"), dict):
                raise ValueError("Invalid resume journal")
        recipe = {"manifest": manifest, "binary_sha256": binary_hash, "runner_sha256": digest(read_file(pathlib.Path(__file__)))}
        for step in manifest["steps"]:
            sid = step["id"]
            files = [("prompt_file", source.parent / step["prompt_file"]), ("schema", source.parent / step["schema"])]
            if step.get("system_file"):
                files.append(("system_file", source.parent / step["system_file"]))
            inputs = [out / (value[5:] + ".json") if value.startswith("step:") else source.parent / value for value in step.get("inputs", [])]
            files.extend(("input", p) for p in inputs)
            snapshots = [(kind, str(path), read_file(path, (1 << 20) if kind == "schema" else MAX_INPUT)) for kind, path in files]
            if sum(len(data) for _, _, data in snapshots) > MAX_INPUT:
                raise ValueError(f"Step {sid} combined input exceeds 64 MiB")
            fingerprint = digest(json.dumps({"recipe": recipe, "step": sid, "files": [(kind, path, digest(data)) for kind, path, data in snapshots]}, sort_keys=True).encode())
            answer_path, result_path = out / (sid + ".json"), out / (sid + ".result.json")
            previous = journal["steps"].get(sid, {})
            if args.resume and previous.get("status") == "ok" and previous.get("fingerprint") == fingerprint:
                if answer_path.is_file() and result_path.is_file() and digest(read_file(answer_path)) == previous.get("answer_sha256") and digest(read_file(result_path)) == previous.get("result_sha256"):
                    print(f"{sid}: resumed", flush=True)
                    continue
            # Use the exact fingerprinted bytes. Changes to source files during
            # a call cannot produce an artifact attributed to an earlier hash.
            with tempfile.TemporaryDirectory(prefix=".inputs-", dir=out) as staging:
                prepared = []
                for index, (kind, _, data) in enumerate(snapshots):
                    target = pathlib.Path(staging) / str(index)
                    target.write_bytes(data)
                    prepared.append((kind, target.name))
                cmd = [str(binary), "--" + manifest["provider"], "--api", manifest["api"], "--model", manifest["model"],
                       "--no-stdin", "--result-json", "--max-tokens", str(manifest["max_tokens"]), "--timeout", str(manifest["timeout_seconds"]) + "s"]
                if manifest.get("key_env"):
                    cmd += ["--key-env", manifest["key_env"]]
                for kind, target in prepared:
                    cmd += [{"prompt_file": "--prompt-file", "system_file": "--system-file", "schema": "--validate-schema", "input": "--file"}[kind], str(target)]
                try:
                    proc = subprocess.run(cmd, cwd=staging, capture_output=True, timeout=manifest["timeout_seconds"] + 5)
                    result = json.loads(proc.stdout) if proc.stdout else {"version": 1, "status": "error", "error": {"code": "cli_error", "message": proc.stderr.decode(errors="replace").strip()}}
                    valid = proc.returncode == 0 and result.get("version") == 1 and result.get("status") == "ok" and result.get("schema_valid") is True
                    answer = json.loads(result["answer"]) if valid else None
                except (subprocess.TimeoutExpired, ValueError, KeyError, TypeError) as exc:
                    result = {"version": 1, "status": "error", "error": {"code": "runner_error", "message": str(exc)}}
                    valid = False
            result_bytes = json_bytes(result)
            atomic_write(result_path, result_bytes)
            record = {"fingerprint": fingerprint, "status": "ok" if valid else "error", "result_sha256": digest(result_bytes)}
            if valid:
                answer_bytes = json_bytes(answer)
                atomic_write(answer_path, answer_bytes)
                record["answer_sha256"] = digest(answer_bytes)
            journal["steps"][sid] = record
            atomic_write(journal_path, json_bytes(journal))
            if not valid:
                raise ValueError(f"Step {sid} failed; inspect {result_path}. No retry or later step was run.")
            print(f"{sid}: saved", flush=True)
    finally:
        lock.unlink()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest")
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--callm", default="callm")
    parser.add_argument("--resume", action="store_true")
    args = parser.parse_args()
    try:
        run(args)
    except (OSError, ValueError, TypeError) as exc:
        print(f"mini-harness: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
