"""Release verification and rollback exercise without Docker credentials."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location('deployment', Path(__file__).resolve().parents[1] / 'deployments/demo/deploy.py')
deploy = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(deploy)


class Fake(deploy.Deployment):
    def __init__(self, root):
        super().__init__(root)
        self.commands = []
        self.failed_port = None
        self.fail_gateway = False

    def command(self, *args, env=None, check=True):
        self.commands.append(args)
        return subprocess.CompletedProcess(args, 0, '', '')

    def prepare(self, info):
        self.compose(info, 'up', '-d')

    def health(self, port, info):
        if port == self.failed_port:
            raise RuntimeError('unhealthy')

    def gateway(self, source):
        if self.fail_gateway:
            raise RuntimeError('gateway unavailable')
        file = self.root / 'gateway/upstream.inc'
        if not file.exists():
            deploy.atomic(file, 'return 503;\n')


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name)
        self.app = Fake(self.path / 'app')

    def bundle(self, version='v1.0.0', run='1-1'):
        folder = self.path / run
        folder.mkdir()
        data = {'version': version, 'commit': 'a'*40, 'deployment_id': run, 'fixture_date': '2026-09-27'}
        for kind in ('api', 'web'):
            data[kind+'_image'] = f'avari-p3-{kind}:'+'a'*40
            data[kind+'_image_id'] = 'sha256:'+'b'*64
        (folder / 'release.json').write_text(json.dumps(data))
        for name in deploy.FILES - {'release.json'}:
            (folder / name).write_text('fixture')
        sums = ''.join(hashlib.sha256((folder / name).read_bytes()).hexdigest()+'  '+name+'\n' for name in sorted(deploy.FILES))
        (folder / 'SHA256SUMS').write_text(sums)
        archive = self.path / (run+'.tgz')
        with tarfile.open(archive, 'w:gz') as tar:
            for name in deploy.FILES | {'SHA256SUMS'}:
                tar.add(folder / name, arcname=name)
        return archive

    def test_switch_and_rollback_preserve_both_volumes(self):
        self.app.apply(self.bundle())
        original = self.app.state()['current']
        self.app.apply(self.bundle('v1.0.1', '2-1'))
        self.assertEqual(self.app.state()['previous'], original)
        self.app.rollback()
        self.assertEqual(self.app.state()['current'], original)
        self.assertFalse(any(command[:2] == ('volume', 'rm') for command in self.app.commands))

    def test_failed_candidate_preserves_current(self):
        self.app.apply(self.bundle())
        state = self.app.state()
        self.app.failed_port = 14802
        with self.assertRaises(RuntimeError):
            self.app.apply(self.bundle('v1.0.1', '2-1'))
        self.assertEqual(self.app.state(), state)
        deleted = [item[2] for item in self.app.commands if item[:2] == ('volume', 'rm')]
        self.assertNotIn(state['current']['volume'], deleted)
        self.assertEqual(len(deleted), 1)

    def test_failed_switch_restores_gateway_and_state(self):
        self.app.apply(self.bundle())
        state = self.app.state()
        include = (self.app.root / 'gateway/upstream.inc').read_text()
        self.app.failed_port = 14800
        with self.assertRaises(RuntimeError):
            self.app.apply(self.bundle('v1.0.1', '2-1'))
        self.assertEqual(self.app.state(), state)
        self.assertEqual((self.app.root / 'gateway/upstream.inc').read_text(), include)

    def test_gateway_failure_cleans_candidate(self):
        self.app.fail_gateway = True
        with self.assertRaises(RuntimeError):
            self.app.apply(self.bundle())
        self.assertIsNone(self.app.state()['current'])
        self.assertTrue(any(command[-1] == 'down' for command in self.app.commands))

    def test_repeat_active_release_does_not_prepare(self):
        bundle = self.bundle()
        self.app.apply(bundle)
        self.app.commands.clear()
        self.app.apply(bundle)
        self.assertEqual(self.app.commands, [])

    def test_checksum_corruption_rejected(self):
        bundle = self.bundle()
        (self.path / '1-1/compose.yml').write_text('changed')
        with self.assertRaises(ValueError):
            deploy.verify_bundle(self.path / '1-1')
        self.assertEqual(self.app.commands, [])

    def test_archive_traversal_rejected(self):
        archive = self.path / 'bad.tgz'
        with tarfile.open(archive, 'w:gz') as tar:
            entry = tarfile.TarInfo('../escape')
            entry.size = 1
            tar.addfile(entry, io.BytesIO(b'x'))
        with self.assertRaises(ValueError):
            self.app.apply(archive)
        self.assertFalse((self.path / 'escape').exists())

    def test_compose_uses_immutable_image_ids(self):
        info = self.app.unpack(self.bundle())
        info['port'] = 14801
        with patch.object(self.app, 'command') as command:
            self.app.compose(info, 'up', '-d')
        values = command.call_args.kwargs['env']
        self.assertEqual(values['API_IMAGE'], 'sha256:'+'b'*64)
        self.assertEqual(values['WEB_IMAGE'], 'sha256:'+'b'*64)

    def test_load_accepts_verified_manifest_id_and_rejects_foreign_id(self):
        data = {'api_image':'api:tag','web_image':'web:tag','api_image_id':'sha256:'+'a'*64,'web_image_id':'sha256:'+'a'*64,'api_manifest_ids':['sha256:'+'b'*64],'web_manifest_ids':['sha256:'+'b'*64]}
        with patch.object(self.app, 'command', return_value=subprocess.CompletedProcess([],0,'sha256:'+'b'*64+'\n','')):
            identities = self.app.load(self.path, data)
        self.assertEqual(identities['web_runtime_image_id'], 'sha256:'+'b'*64)
        with patch.object(self.app, 'command', return_value=subprocess.CompletedProcess([],0,'sha256:'+'c'*64+'\n','')):
            with self.assertRaises(ValueError):
                self.app.load(self.path, data)

    def test_nonroot_compose_variables_cross_sudo(self):
        with patch.object(deploy.os, 'geteuid', return_value=1000), patch.object(deploy.subprocess, 'run', return_value=subprocess.CompletedProcess([],0,'','')) as run:
            app = deploy.Deployment(self.path / 'nonroot')
            app.command('compose', 'config', env={'API_IMAGE':'avari-p3-api:'+ 'a'*40})
        command = run.call_args.args[0]
        self.assertEqual(command[:3], ['sudo', '-n', 'env'])
        self.assertIn('API_IMAGE=avari-p3-api:'+'a'*40, command)
        self.assertEqual(command[-3:], ['docker','compose','config'])


if __name__ == '__main__':
    unittest.main()
