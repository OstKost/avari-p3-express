#!/usr/bin/env python3
"""Assemble the local plugin from canonical skills; never copy credentials."""
import argparse
import json
from pathlib import Path
import re
import shutil
from urllib.parse import urlparse

ROOT = Path(__file__).resolve().parents[1]
SKILLS = ("avari-p3-cycle", "avari-task-design", "avari-agent-work", "avari-quality-review", "avari-retrospective")


def build(output, binary, api_url="http://127.0.0.1:4820", root=ROOT):
    output, binary, root = Path(output).resolve(), Path(binary).resolve(), Path(root).resolve()
    u = urlparse(api_url)
    if u.scheme != "http" or u.hostname not in {"localhost", "127.0.0.1", "::1"} or u.username or u.password or u.query or u.fragment or u.path not in {"", "/"}:
        raise ValueError("API URL must be a loopback HTTP origin")
    if not binary.is_file():
        raise ValueError("Build the MCP binary first")
    # Only generated harness output is replaceable; source and home config are not.
    artifacts = (root / ".harness/artifacts").resolve()
    if not output.is_relative_to(artifacts) or output == artifacts:
        raise ValueError("Output must be a child of .harness/artifacts")
    staging = output.with_name(output.name + ".staging")
    if staging.exists():
        shutil.rmtree(staging)
    staging.mkdir(parents=True)
    try:
        manifest = json.loads((root / "plugins/avari-workspace/.codex-plugin/plugin.json").read_text())
        (staging / ".codex-plugin").mkdir()
        (staging / ".codex-plugin/plugin.json").write_text(json.dumps(manifest, indent=2) + "\n")
        for name in SKILLS:
            source = root / ".agents/skills" / name
            target = staging / "skills" / name
            target.mkdir(parents=True)
            # Only instruction resources; no logs, environments, binaries or credentials.
            for item in source.rglob("*.md"):
                if item.is_symlink():
                    raise ValueError("Skill resource symlinks are not packaged")
                dest = target / item.relative_to(source)
                dest.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(item, dest)
        config = {"mcpServers": {"avari": {"command": str(binary), "env": {"AVARI_API_URL": api_url}}}}
        (staging / ".mcp.json").write_text(json.dumps(config, indent=2) + "\n")
        shutil.copyfile(root / "plugins/avari-workspace/README.md", staging / "README.md")
        validate(staging)
        if output.exists():
            shutil.rmtree(output)
        staging.rename(output)
    finally:
        if staging.exists():
            shutil.rmtree(staging)
    return output


def validate(package):
    package = Path(package)
    manifest = json.loads((package / ".codex-plugin/plugin.json").read_text())
    if manifest["name"] != "avari-workspace" or manifest["skills"] != "./skills/" or manifest["mcpServers"] != "./.mcp.json":
        raise ValueError("Invalid plugin manifest")
    actual = {p.name for p in (package / "skills").iterdir() if p.is_dir()}
    if actual != set(SKILLS):
        raise ValueError("Plugin must contain exactly the canonical runtime skills")
    for name in SKILLS:
        if not (package / "skills" / name / "SKILL.md").is_file():
            raise ValueError("Missing skill")
    for file in package.rglob("*.md"):
        for target in re.findall(r"\]\(([^)]+)\)", file.read_text()):
            if re.match(r"[a-z]+://|#", target):
                continue
            dest = (file.parent / target.split("#", 1)[0]).resolve()
            if not dest.is_relative_to(package.resolve()) or not dest.is_file():
                raise ValueError("Broken or escaping instruction reference")
    config = json.loads((package / ".mcp.json").read_text())
    server = config["mcpServers"]["avari"]
    if set(server) != {"command", "env"} or set(server["env"]) != {"AVARI_API_URL"}:
        raise ValueError("Only the API origin may be configured; credentials are inherited")
    return manifest


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=ROOT / ".harness/artifacts/avari-workspace")
    parser.add_argument("--mcp-bin", type=Path, required=True)
    parser.add_argument("--api-url", default="http://127.0.0.1:4820")
    args = parser.parse_args()
    print(build(args.output, args.mcp_bin, args.api_url))
