# Avari Workspace 0.1.0

This local plugin combines project decision briefs, assignment drafting, execution, independent QA and retrospectives with the Avari stdio MCP. It does not run agents inside Avari, accept tasks, synchronize trackers or publish code.

The repository stores canonical instructions. Build an installable local package with the repository's `scripts/build_agent_plugin.py --mcp-bin <absolute binary path>` after building `apps/api/cmd/mcp`. The source scaffold alone is not the assembled plugin.

Use the generated package's .mcp.json in a compatible local client. Its command is the configured binary's absolute path; AVARI_API_URL is a loopback HTTP origin. Supply AVARI_AGENT_TOKEN through the MCP process environment, separately for executor and reviewer clients. Never put credentials in this package, its manifest, source control or logs. The manager key is not an agent token.

The package contains five skills. For custom role discovery use the repository's .codex/agents on clients that support it; other clients can explicitly load the relevant SKILL.md. Role instructions are not automatically registered by this plugin. Keep manager decisions and OS sandbox isolation separate from API role enforcement.
