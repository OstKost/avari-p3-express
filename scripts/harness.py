#!/usr/bin/env python3
"""Small local/CI harness. Python 3.11+, standard library only."""
import argparse
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import tempfile
import time
import tomllib

ROOT = Path(__file__).resolve().parents[1]
GATES = {
    "api": [
        ("go-format", ["gofmt", "-l", "."], "apps/api", True, {}),
        ("go-vet", ["go", "vet", "./..."], "apps/api", False, {}),
        ("go-race", ["go", "test", "-race", "-cover", "./..."], "apps/api", False, {}),
        ("go-fixture-build", ["go", "build", "-o", os.devnull, "./cmd/demo"], "apps/api", False, {"CGO_ENABLED": "0"}),
        ("go-build", ["go", "build", "-o", os.devnull, "./cmd/server"], "apps/api", False, {"CGO_ENABLED": "0"}),
    ],
    "web": [
        ("web-lint", ["pnpm", "run", "lint"], "apps/web", False, {}),
        ("web-build", ["pnpm", "run", "build"], "apps/web", False, {}),
    ],
    "browser": [
        ("browser-workflow", ["pnpm", "run", "test:e2e"], "apps/web", False, {}),
    ],
    "smoke": [
        ("demo-fixtures", [sys.executable, "-m", "unittest", "discover", "-s", "scripts", "-p", "test_demo.py"], ".", False, {}),
        ("browser-smoke", ["pnpm", "run", "test:smoke"], "apps/web", False, {}),
    ],
    "harness": [
        ("harness-metadata", [sys.executable, "scripts/harness.py", "validate"], ".", False, {}),
        ("harness-tests", [sys.executable, "-m", "unittest", "discover", "-s", "scripts", "-p", "test_*.py"], ".", False, {}),
    ],
}


def git_snapshot(root):
    result = subprocess.run(
        ["git", "status", "--short", "--untracked-files=normal"],
        cwd=root, capture_output=True, text=True, check=True,
    )
    head = subprocess.run(
        ["git", "rev-parse", "--verify", "HEAD"],
        cwd=root, capture_output=True, text=True,
    )
    return {"head": head.stdout.strip() if head.returncode == 0 else None,
            "status": result.stdout.splitlines()}


def context(root):
    snapshot = git_snapshot(root)
    print("Implemented: Go/SQLite P3.express & SDLC / AI-SDLC Project Management + React UI")
    print("Product specs: _init/INDEX.md")
    print("Instructions: AGENTS.md; apps/api/AGENTS.md; apps/web/AGENTS.md; _init/AGENTS.md")
    print("Map: docs/architecture.md | Usage: docs/harness.md")
    print("Checks: make check-api / check-web / check-harness / check")
    print("HEAD:", snapshot["head"] or "none (unborn branch; inspect untracked files)")
    print("Working tree (at most 30 entries):")
    print("\n".join(snapshot["status"][:30]) or "clean")
    if len(snapshot["status"]) > 30:
        print(f"... {len(snapshot['status']) - 30} more entries; use git status for details")


def validate(root):
    errors = []
    rules = [root / p for p in ("AGENTS.md", "apps/api/AGENTS.md", "apps/web/AGENTS.md", "_init/AGENTS.md")]
    skills = sorted((root / ".agents/skills").glob("*/SKILL.md"))
    agents = sorted((root / ".codex/agents").glob("*.toml"))
    if not skills or not agents:
        errors.append("Expected discoverable skills and custom agents")
    for path in rules + skills:
        if not path.is_file():
            errors.append(f"Missing {path.relative_to(root)}")
            continue
        budget = 7000 if path == root / "AGENTS.md" else 4000
        if path.stat().st_size > budget:
            errors.append(f"{path.relative_to(root)} exceeds {budget} bytes; move optional detail to docs")
    names = set()
    for path in skills:
        content = path.read_text()
        match = re.match(r"\A---\n(.*?)\n---\n", content, re.S)
        fields = {}
        if match:
            # This repository deliberately uses only plain, single-line YAML scalars.
            for line in match[1].splitlines():
                key, separator, value = line.partition(": ")
                if not separator or key in fields or key not in {"name", "description"}:
                    errors.append(f"{path.relative_to(root)}: use unique name/description plain scalars")
                fields[key] = value
        name = fields.get("name", "")
        description = fields.get("description", "")
        if not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", name) or len(name) > 64 or name != path.parent.name:
            errors.append(f"{path.relative_to(root)}: invalid or mismatched name")
        if name in names:
            errors.append(f"Duplicate skill {name}")
        names.add(name)
        if not description or len(description) > 350 or ": " in description or description[0] in "|>{[\"'":
            errors.append(f"{path.relative_to(root)}: description must be a plain scalar of 1–350 characters")
    names = set()
    for path in [root / ".codex/config.toml"] + agents:
        try:
            data = tomllib.loads(path.read_text())
            if path in agents:
                for key in ("name", "description", "developer_instructions"):
                    if not isinstance(data.get(key), str) or not data[key].strip():
                        errors.append(f"{path.relative_to(root)}: missing {key}")
                if data.get("name") != path.stem or data.get("name") in names:
                    errors.append(f"{path.relative_to(root)}: mismatched or duplicate agent name")
                names.add(data.get("name"))
            elif data.get("agents", {}).get("max_concurrent_threads_per_session") != 2:
                errors.append("Project convention: at most two concurrent child agents")
        except (OSError, ValueError) as error:
            errors.append(f"{path.relative_to(root)}: {error}")
    docs = rules + skills + sorted((root / "docs").rglob("*.md"))
    for path in docs:
        if not path.exists():
            continue
        for target in re.findall(r"\]\(([^)]+)\)", path.read_text()):
            if re.match(r"[a-z]+://|#", target):
                continue
            target = target.split("#", 1)[0]
            if not (path.parent / target).exists():
                errors.append(f"{path.relative_to(root)}: broken local link {target}")
    if errors:
        print("\n".join(errors))
        return 1
    print(f"PASS: {len(rules)} instruction files, {len(skills)} skills, {len(agents)} agents; local links and size budgets")
    print("Static validation only; runtime discovery and agent behavior need a real session.")
    return 0


def stop_process(process):
    # Demo supervisors start their own sessions, so killpg alone misses them.
    # Snapshot only this runner's descendants; never use global pkill patterns.
    if os.name == "posix":
        try:
            try:
                os.killpg(process.pid, signal.SIGSTOP)
            except ProcessLookupError:
                pass
            tree = subprocess.run(["ps", "-axo", "pid=,ppid="], capture_output=True, text=True, check=True, timeout=5)
            parents = {int(pid): int(parent) for pid, parent in (line.split() for line in tree.stdout.splitlines())}
            descendants = []
            def visit(parent):
                for pid, ppid in parents.items():
                    if ppid == parent:
                        visit(pid)
                        descendants.append(pid)
            visit(process.pid)
            for pid in descendants:
                try:
                    os.kill(pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
        finally:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait()
    else:
        process.kill()
        process.wait()


def run_step(root, log_dir, step, timeout=600):
    name, command, directory, require_empty, extra_env = step
    log_path = log_dir / f"{name}.log"
    started = time.monotonic()
    with log_path.open("w") as log:
        try:
            process = subprocess.Popen(command, cwd=root / directory,
                                       env={**os.environ, **extra_env}, stdout=log,
                                       stderr=subprocess.STDOUT,
                                       start_new_session=os.name == "posix")
            try:
                code = process.wait(timeout=timeout)
            except subprocess.TimeoutExpired:
                stop_process(process)
                log.write(f"\nTimed out after {timeout}s; stopped the check process group\n")
                code = 124
            except KeyboardInterrupt:
                stop_process(process)
                raise
        except OSError as error:
            log.write(f"\n{error}\n")
            code = 1
    if require_empty and log_path.stat().st_size:
        code = code or 1
    passed = code == 0
    seconds = round(time.monotonic() - started, 2)
    print(f"{'PASS' if passed else 'FAIL'} {name} ({seconds}s)", flush=True)
    if not passed:
        print("\n".join(log_path.read_text(errors="replace").splitlines()[-40:]))
        print(f"Full log: {log_path}")
    return {"name": name, "passed": passed, "exit_code": code,
            "seconds": seconds, "log": str(log_path), "command": command}


def check(root, scope):
    runs = root / ".harness/runs"
    runs.mkdir(parents=True, exist_ok=True)
    log_dir = Path(tempfile.mkdtemp(prefix=f"{scope}-", dir=runs))
    scopes = ("harness", "api", "web", "browser") if scope == "full" else (("harness", "api", "web") if scope == "all" else (scope,))
    versions = {"python": sys.version.split()[0]}
    for tool in ("go", "node", "pnpm"):
        try:
            result = subprocess.run([tool, "version" if tool == "go" else "--version"], capture_output=True, text=True, timeout=10)
            versions[tool] = result.stdout.strip() if result.returncode == 0 else "unavailable"
        except (OSError, subprocess.TimeoutExpired):
            versions[tool] = "unavailable"
    (log_dir / "environment.json").write_text(json.dumps({"versions": versions, "scope": scope, "fixture_date": "2026-09-27", "fixture_version": 1, "platform": sys.platform}, indent=2) + "\n")
    results = []
    print(f"Scope: {scope}. Logs: {log_dir}", flush=True)
    for group in scopes:
        for step in GATES[group]:
            results.append(run_step(root, log_dir, step))
    (log_dir / "summary.json").write_text(json.dumps(results, indent=2) + "\n")
    failures = sum(not result["passed"] for result in results)
    print(f"{len(results) - failures}/{len(results)} checks passed")
    return 1 if failures else 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("context")
    commands.add_parser("validate")
    check_parser = commands.add_parser("check")
    check_parser.add_argument("scope", choices=("all", "full", *GATES), default="all", nargs="?")
    args = parser.parse_args()
    if args.command == "context":
        context(ROOT)
        return 0
    if args.command == "validate":
        return validate(ROOT)
    return check(ROOT, args.scope)


if __name__ == "__main__":
    sys.exit(main())
