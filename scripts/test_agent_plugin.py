import json
from pathlib import Path
import tempfile
import unittest
from build_agent_plugin import ROOT, SKILLS, build, validate


class PluginBuildTests(unittest.TestCase):
    def test_canonical_resources_and_no_credentials(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            source = ROOT / "plugins/avari-workspace"
            import shutil
            shutil.copytree(source, root / "plugins/avari-workspace")
            for name in SKILLS:
                shutil.copytree(ROOT / ".agents/skills" / name, root / ".agents/skills" / name)
            binary = root / "mcp"
            binary.write_text("binary fixture")
            output = root / ".harness/artifacts/avari-workspace"
            build(output, binary, root=root)
            validate(output)
            for name in SKILLS:
                self.assertEqual((output / "skills" / name / "SKILL.md").read_bytes(), (ROOT / ".agents/skills" / name / "SKILL.md").read_bytes())
            config = json.loads((output / ".mcp.json").read_text())
            self.assertNotIn("AVARI_AGENT_TOKEN", config["mcpServers"]["avari"].get("env", {}))
            self.assertEqual(config["mcpServers"]["avari"]["command"], str(binary.resolve()))
            build(output, binary, root=root)  # deterministic replacement
            validate(output)

    def test_output_and_url_boundaries(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            binary = root / "mcp"
            binary.touch()
            for url in ["https://localhost", "http://example.com", "http://secret@localhost", "http://localhost?token=secret", "http://localhost/path"]:
                with self.assertRaises(ValueError):
                    build(root / ".harness/artifacts/plugin", binary, url, root)
            with self.assertRaises(ValueError):
                build(root / ".agents", binary, root=root)


if __name__ == "__main__":
    unittest.main()
