import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec=importlib.util.spec_from_file_location("sync_toolkit",Path(__file__).resolve().parents[1]/"scripts/sync_toolkit.py")
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)

class ToolkitUpdatesTest(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.source=Path(self.temp.name)/"source";self.target=Path(self.temp.name)/"project/.factory"
        self.target.parent.mkdir(parents=True)
        for name,text in {"orchestrator/session.py":"v1", "resources/skill.md":"original", "factories/team/factory.json":"team config", "resources/config.local.json":"secret", "orchestrator/comms/.env.production":"secret"}.items():
            p=self.source/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text)
    def test_install_seeds_factories_but_update_preserves_team_config(self):
        module.synchronize(self.source,self.target,True)
        self.assertFalse((self.target/"resources/config.local.json").exists())
        self.assertFalse((self.target/"orchestrator/comms/.env.production").exists())
        (self.target/"factories/team/factory.json").write_text("custom")
        (self.source/"orchestrator/session.py").write_text("v2")
        preview=module.synchronize(self.source,self.target)
        self.assertEqual(len(preview["actions"]),1)
        self.assertEqual((self.target/"orchestrator/session.py").read_text(),"v1")
        module.synchronize(self.source,self.target,True)
        self.assertEqual((self.target/"orchestrator/session.py").read_text(),"v2")
        self.assertEqual((self.target/"factories/team/factory.json").read_text(),"custom")
    def test_conflict_does_not_partially_update_and_local_only_edits_survive(self):
        module.synchronize(self.source,self.target,True)
        (self.target/"resources/skill.md").write_text("local")
        result=module.synchronize(self.source,self.target,True)
        self.assertEqual(result["preserved_local_edits"],["resources/skill.md"])
        (self.source/"resources/skill.md").write_text("upstream")
        (self.source/"orchestrator/session.py").write_text("v2")
        result=module.synchronize(self.source,self.target,True)
        self.assertEqual(result["mode"],"blocked")
        self.assertEqual((self.target/"orchestrator/session.py").read_text(),"v1")
    def test_removed_managed_files_and_untracked_files(self):
        module.synchronize(self.source,self.target,True)
        (self.source/"resources/skill.md").unlink()
        (self.target/"resources/extra.md").write_text("local extra")
        module.synchronize(self.source,self.target,True)
        self.assertFalse((self.target/"resources/skill.md").exists())
        self.assertTrue((self.target/"resources/extra.md").exists())
    def test_symlink_and_tampered_manifest_cannot_target_team_files(self):
        module.synchronize(self.source,self.target,True)
        manifest=self.target/".toolkit.json"
        manifest.write_text(json.dumps({"version":1,"files":{"factories/team/factory.json":"a"*64}}))
        with self.assertRaises(ValueError):module.synchronize(self.source,self.target,True)
        manifest.unlink()
        (self.target/"orchestrator/session.py").unlink()
        (self.target/"orchestrator/session.py").symlink_to(self.source/"orchestrator/session.py")
        with self.assertRaises(ValueError):module.synchronize(self.source,self.target,True)
