from contextlib import closing, redirect_stdout
from http.cookiejar import CookieJar
import fcntl
import io
import json
import os
from pathlib import Path
import re
import signal
import socket
import sqlite3
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch
import urllib.request

import demo
import harness

ROOT = Path(__file__).resolve().parents[1]


class DemoTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name) / 'demo'

    def run_demo(self, command, directory=None, *extra, success=True):
        result = subprocess.run([sys.executable, str(ROOT / 'scripts/demo.py'), command, '--dir', str(directory or self.directory), *extra], capture_output=True, text=True, timeout=150)
        if success:
            self.assertEqual(result.returncode, 0, result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0)
        return result

    def paths(self, directory=None):
        return {key: Path(value) for key, value in json.loads(self.run_demo('paths', directory).stdout).items()}

    def seed(self, profile='smoke'):
        self.run_demo('seed', self.directory, '--profile', profile)
        return self.paths()

    def manifest(self):
        return json.loads(self.paths()['manifest'].read_text())

    def snapshot(self, path):
        # Domain values and relationships, excluding generated UUID/audit time.
        with closing(sqlite3.connect(path)) as db:
            coords = {row[0]: list(row[1:]) for row in db.execute('SELECT s.id,p.name,c.phase_code,c.number,s.code FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id JOIN p3_projects p ON p.id=c.project_id')}
            stages = {row[0]: list(row[1:]) for row in db.execute('SELECT s.id,p.name,s.code FROM p3_sdlc_stages s JOIN p3_projects p ON p.id=s.project_id')}
            ignored = {'id', 'project_id', 'task_id', 'run_id', 'result_id', 'submission_digest', 'submit_fingerprint', 'last_message_at', 'verification_report_id'}
            def normalize(value):
                if isinstance(value, dict):
                    return {key: normalize(item) for key, item in value.items() if key not in ignored and not key.endswith('_at')}
                if isinstance(value, list):
                    return [normalize(item) for item in value]
                if isinstance(value, str):
                    return coords.get(value, stages.get(value, value))
                return value
            queries = {
                'projects': 'SELECT name,description,due_date,archived,rag_status,progress,current_phase FROM p3_projects ORDER BY name',
                'steps': 'SELECT p.name,c.phase_code,c.number,s.code,s.status,s.due_date,ch.text,ch.done FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id JOIN p3_projects p ON p.id=c.project_id LEFT JOIN p3_checklist ch ON ch.step_id=s.id ORDER BY p.name,c.phase_code,c.number,s.code,ch.text',
                'stages': 'SELECT p.name,s.code,s.status,s.progress FROM p3_sdlc_stages s JOIN p3_projects p ON p.id=s.project_id ORDER BY p.name,s.code',
                'actions': 'SELECT p.name,a.title,a.due_date,a.priority,a.done FROM p3_actions a JOIN p3_projects p ON p.id=a.project_id ORDER BY p.name,a.title',
                'blockers': 'SELECT p.name,b.title,b.description,b.resolved FROM p3_blockers b JOIN p3_projects p ON p.id=b.project_id ORDER BY p.name,b.title',
                'links': 'SELECT title,url,description FROM p3_links ORDER BY title,url',
                'comments': 'SELECT text FROM p3_comments ORDER BY text',
                'articles': 'SELECT title,category,summary,body FROM p3_articles ORDER BY title',
            }
            result = {key: db.execute(query).fetchall() for key, query in queries.items()}
            result['tasks'] = sorted([normalize(json.loads(row[0])) for row in db.execute('SELECT body FROM work_tasks')], key=lambda item: (item['goal'], item['due_date']))
            return result

    def test_lifecycle_and_reproducibility(self):
        paths = self.seed('demo')
        original = self.manifest()
        self.assertEqual(original['schema_version'], 2)
        before = self.snapshot(paths['db'])
        with closing(sqlite3.connect(paths['db'])) as db:
            self.assertEqual(db.execute('SELECT count(*) FROM p3_projects').fetchone()[0], 3)
            self.assertEqual(db.execute('SELECT count(*) FROM work_tasks').fetchone()[0], 21)
            bodies = [json.loads(row[0]) for row in db.execute('SELECT body FROM work_tasks')]
            self.assertEqual({body['status'] for body in bodies}, {'draft', 'ready', 'in_progress', 'in_review', 'done'})
            self.assertEqual(sum(bool(body['blocker']) for body in bodies), 3)
            self.assertEqual(sum(any(run.get('review') and run['review']['decision'] == 'return' for run in body['runs']) for body in bodies), 3)
            self.assertEqual(db.execute('PRAGMA integrity_check').fetchall(), [('ok',)])
            self.assertEqual(db.execute('PRAGMA foreign_key_check').fetchall(), [])
            states = {body['id']: body['status'] for body in bodies}
            for key, task_id in original['tasks'].items():
                self.assertEqual(states[task_id], original['expected_task_states'][key])
        self.run_demo('seed')
        self.assertEqual(self.paths(), paths)
        self.run_demo('seed', self.directory, '--date', '2026-01-01', success=False)
        self.run_demo('reset', self.directory, '--date', 'invalid', success=False)
        self.assertEqual(self.snapshot(paths['db']), before)
        self.assertEqual(self.manifest(), original)
        self.run_demo('reset')
        new = self.manifest()
        self.assertNotEqual(new['projects'], original['projects'])
        self.assertEqual(self.snapshot(self.paths()['db']), before)
        note = self.directory / 'user-note.txt'
        note.write_text('keep')
        self.run_demo('clean')
        self.run_demo('clean')
        self.assertEqual(note.read_text(), 'keep')
        self.assertFalse((self.directory / 'current.json').exists())
        self.assertEqual(list((self.directory / 'generations').glob('*/demo.db')), [])
        self.seed()

    def test_unowned_symlink_and_corrupt_guards(self):
        paths = self.seed()
        victim = Path(self.temporary.name) / 'real-data'
        victim.write_text('keep')
        unknown = Path(self.temporary.name) / 'unknown'
        unknown.mkdir()
        (unknown / 'demo.db').write_text('real db')
        self.run_demo('reset', unknown, success=False)
        for root, name in [(self.directory, '.lock'), (self.directory, 'manager.key'), (self.directory, 'current.json'), (paths['db'].parent, 'manifest.json'), (paths['db'].parent, 'demo.db')]:
            file = root / name
            saved = file.read_bytes() if file.exists() else None
            file.unlink(missing_ok=True)
            file.symlink_to(victim)
            self.run_demo('seed', self.directory, '--profile', 'smoke', success=False)
            self.run_demo('api', success=False)
            self.assertEqual(victim.read_text(), 'keep')
            file.unlink()
            if saved is not None:
                file.write_bytes(saved)
        manifest = json.loads(paths['manifest'].read_text())
        manifest['projects']['shop'] = 'nonexistent'
        paths['manifest'].write_text(json.dumps(manifest))
        self.run_demo('seed', self.directory, '--profile', 'smoke', success=False)
        self.run_demo('api', success=False)
        self.run_demo('reset', self.directory, '--profile', 'smoke')
        repaired = self.paths()
        repaired['db'].write_bytes(b'corrupt database')
        self.run_demo('paths', success=False)
        self.run_demo('reset', self.directory, '--profile', 'smoke')
        self.paths()
        self.assertEqual((unknown / 'demo.db').read_text(), 'real db')

    def test_publication_failures_leave_complete_generation(self):
        self.seed()
        original_paths = self.paths()
        original_manifest = self.manifest()
        writer = demo.write_json
        # Before and after the commit point; also manifest creation failure.
        for failure in ('manifest.json', 'before-current', 'after-current'):
            def failing(path, value):
                if path.name == failure or (path.name == 'current.json' and failure == 'before-current'):
                    raise OSError('controlled write failure')
                writer(path, value)
                if path.name == 'current.json' and failure == 'after-current':
                    raise OSError('controlled post-publication failure')
            with patch.object(sys, 'argv', ['demo.py', 'reset', '--dir', str(self.directory), '--profile', 'smoke']), patch.object(demo, 'write_json', side_effect=failing):
                with self.assertRaises(SystemExit):
                    demo.main()
            active = self.paths()
            if failure != 'after-current':
                self.assertEqual(active, original_paths)
                self.assertEqual(self.manifest(), original_manifest)
            else:
                self.assertNotEqual(active, original_paths)
                self.assertEqual(self.snapshot(active['db']), self.snapshot(original_paths['db']))

    def test_lock_rejects_competing_mutations(self):
        paths = self.seed()
        before = self.snapshot(paths['db'])
        with (self.directory / '.lock').open('a') as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            for command in ('seed', 'reset', 'clean'):
                self.run_demo(command, success=False)
        self.assertEqual(self.snapshot(paths['db']), before)

    def test_legacy_is_never_silently_overwritten(self):
        self.run_demo('clean')
        (self.directory / 'demo.db').write_bytes(b'legacy data')
        self.run_demo('seed', success=False)
        self.run_demo('api', success=False)
        self.run_demo('reset', self.directory, '--profile', 'smoke')
        self.assertEqual((self.directory / 'demo.db').read_bytes(), b'legacy data')
        self.paths()
        self.run_demo('clean')
        self.assertFalse((self.directory / 'demo.db').exists())

    def test_backup_restore_includes_wal(self):
        paths = self.seed()
        backup = Path(self.temporary.name) / 'backup'
        with closing(sqlite3.connect(paths['db'])) as db:
            db.execute('PRAGMA journal_mode=WAL')
            db.execute('PRAGMA wal_autocheckpoint=0')
            step = next(iter(self.manifest()['steps'].values()))
            db.execute('INSERT INTO p3_comments(id,step_id,text,created_at) VALUES(?,?,?,?)', ('wal-comment', step, 'Committed WAL data', '2026-09-27'))
            db.commit()
            self.assertGreater(Path(str(paths['db']) + '-wal').stat().st_size, 0)
            before = self.snapshot(paths['db'])
            self.run_demo('backup', self.directory, '--to', str(backup))
        self.assertEqual(self.snapshot(self.paths(backup)['db']), before)
        self.assertFalse((backup / 'manager.key').exists())
        self.run_demo('backup', self.directory, '--to', str(backup), success=False)
        self.run_demo('reset', self.directory, '--profile', 'smoke')
        self.run_demo('restore', self.directory, '--from', str(backup))
        self.assertEqual(self.snapshot(self.paths()['db']), before)
        restored = Path(self.temporary.name) / 'new-restored'
        self.run_demo('restore', restored, '--from', str(backup))
        self.assertEqual(self.snapshot(self.paths(restored)['db']), before)

    def start_api(self):
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        log = tempfile.TemporaryFile()
        self.addCleanup(log.close)
        process = subprocess.Popen([sys.executable, str(ROOT / 'scripts/demo.py'), 'api', '--dir', str(self.directory)], env=dict(os.environ, PORT=str(port)), stdout=log, stderr=log, start_new_session=True)
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        url = f'http://127.0.0.1:{port}/healthz'
        deadline = time.monotonic() + 60
        try:
            while True:
                try:
                    with opener.open(url, timeout=0.5) as response:
                        self.assertEqual(response.status, 200)
                    break
                except OSError:
                    if process.poll() is not None or time.monotonic() > deadline:
                        log.seek(0)
                        self.fail(log.read().decode())
                    time.sleep(0.05)
        except BaseException:
            process.terminate()
            try:
                process.wait(timeout=20)
            except subprocess.TimeoutExpired:
                harness.stop_process(process)
            raise
        return process, opener, url, log

    def test_api_signals_and_restart_restored_data(self):
        self.seed()
        for sig in (signal.SIGINT, signal.SIGTERM):
            process, opener, url, _ = self.start_api()
            try:
                self.run_demo('reset', success=False)
                process.send_signal(sig)
                self.assertEqual(process.wait(timeout=20), 0)
                with self.assertRaises(OSError):
                    opener.open(url, timeout=0.5)
                self.paths()
            finally:
                if process.poll() is None:
                    process.terminate()
                    process.wait(timeout=20)

    def test_sigkill_wrapper_keeps_api_lock(self):
        self.seed()
        process, opener, url, log = self.start_api()
        log.seek(0)
        child_pid = int(re.search(r'Demo API PID: (\d+)', log.read().decode())[1])
        try:
            process.kill()
            process.wait(timeout=10)
            with opener.open(url, timeout=1) as response:
                self.assertEqual(response.status, 200)
            self.run_demo('reset', success=False)
            self.run_demo('clean', success=False)
        finally:
            os.kill(child_pid, signal.SIGTERM)
            deadline = time.monotonic() + 15
            while True:
                try:
                    with demo.locked(self.directory):
                        break
                except ValueError:
                    if time.monotonic() > deadline:
                        os.kill(child_pid, signal.SIGKILL)
                        self.fail('API kept lock after shutdown')
                    time.sleep(0.05)
        with self.assertRaises(OSError):
            opener.open(url, timeout=0.5)
        self.run_demo('reset', self.directory, '--profile', 'smoke')

    def test_bind_and_build_failures_release_lock(self):
        self.seed()
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0))
            listener.listen()
            with patch.dict(os.environ, PORT=str(listener.getsockname()[1])):
                self.run_demo('api', success=False)
        self.paths()
        original_run = demo.run
        def failed_build(command, *args, **kwargs):
            if command[:2] == ['go', 'build']:
                raise RuntimeError('controlled build failure')
            return original_run(command, *args, **kwargs)
        with patch.object(sys, 'argv', ['demo.py', 'api', '--dir', str(self.directory)]), patch.object(demo, 'run', side_effect=failed_build):
            with self.assertRaises(SystemExit):
                demo.main()
        self.paths()

    def test_child_deadline_releases_lock(self):
        self.run_demo('clean')
        with demo.locked(self.directory) as (_, fd):
            with self.assertRaises(TimeoutError):
                demo.run([sys.executable, '-c', 'import time; time.sleep(30)'], fd, timeout=0.1)
        with demo.locked(self.directory):
            pass

    def test_http_manifest_and_restored_state(self):
        self.seed('demo')
        manifest = self.manifest()
        backup = Path(self.temporary.name) / 'backup'
        self.run_demo('backup', self.directory, '--to', str(backup))
        self.run_demo('reset', self.directory, '--profile', 'smoke')
        self.run_demo('restore', self.directory, '--from', str(backup))
        process, _, health, _ = self.start_api()
        base = health.removesuffix('/healthz')
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(CookieJar()))
        try:
            key = (self.directory / 'manager.key').read_text().strip()
            request = urllib.request.Request(base + '/api/session', data=json.dumps({'key': key}).encode(), headers={'Content-Type': 'application/json', 'X-Avari-Local': 'manager'})
            with opener.open(request, timeout=5) as response:
                self.assertEqual(response.status, 200)
            def fetch(endpoint):
                with opener.open(base + endpoint, timeout=5) as response:
                    self.assertEqual(response.status, 200)
                    return json.load(response)
            projects = []
            for project_id in manifest['projects'].values():
                project = fetch('/api/v1/p3/projects/' + project_id)
                projects.append(project)
                self.assertEqual(project['progress'], 0)  # Stored management value, not mean phases.
                for phase in project['phases']:
                    for step in phase['steps']:
                        checklist = step['checklist']
                        expected = sum(item['done'] for item in checklist) * 100 // len(checklist) if checklist else 0
                        self.assertEqual(step['progress'], expected)
                    self.assertEqual(phase['progress'], sum(step['progress'] for step in phase['steps']) // len(phase['steps']))
            for semantic, task_id in manifest['tasks'].items():
                task = fetch('/api/v1/work-tasks/' + task_id)['task']
                self.assertEqual(task['status'], manifest['expected_task_states'][semantic])
                for step_id in task['step_ids']:
                    self.assertIn(step_id, manifest['steps'].values())
                if task['status'] == 'done':
                    run = task['runs'][0]
                    self.assertEqual(run['actor'], 'demo-executor')
                    self.assertEqual(run['verification_reports'][0]['reviewer_id'], 'demo-reviewer')
                    self.assertEqual(run['review']['decision'], 'accept')
            self.assertTrue(all(project['sdlc_stages'][0]['progress'] == 50 for project in projects))
        finally:
            process.terminate()
            self.assertEqual(process.wait(timeout=20), 0)

    def test_process_crash_at_publication(self):
        self.seed()
        before = self.paths()
        original = self.snapshot(before['db'])
        for moment in ('before', 'after'):
            code = """
import os,signal,sys
sys.path.insert(0, sys.argv[1])
import demo
writer=demo.write_json
moment=sys.argv[3]
def crash(path,value):
    if path.name=='current.json' and moment=='before':
        os.kill(os.getpid(),signal.SIGKILL)
    writer(path,value)
    if path.name=='current.json' and moment=='after':
        os.kill(os.getpid(),signal.SIGKILL)
demo.write_json=crash
sys.argv=['demo.py','reset','--dir',sys.argv[2],'--profile','smoke']
demo.main()
"""
            result = subprocess.run([sys.executable, '-c', code, str(ROOT / 'scripts'), str(self.directory), moment], capture_output=True, text=True, timeout=150)
            self.assertEqual(result.returncode, -signal.SIGKILL)
            active = self.paths()
            if moment == 'before':
                self.assertEqual(active, before)
            else:
                self.assertNotEqual(active, before)
            self.assertEqual(self.snapshot(active['db']), original)
        self.run_demo('clean')
        self.assertEqual(list((self.directory / 'generations').glob('*/demo.db')), [])

    def test_harness_timeout_removes_detached_api(self):
        self.seed()
        # Warm the binary build without including cold toolchain time in timeout.
        subprocess.run(['go', 'build', '-o', str(self.directory / 'server'), './cmd/server'], cwd=ROOT / 'apps/api', check=True, timeout=120)
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        logs = Path(self.temporary.name) / 'logs'
        logs.mkdir()
        step = ('api-timeout', [sys.executable, 'scripts/demo.py', 'api', '--dir', str(self.directory)], '.', False, {'PORT': str(port)})
        with redirect_stdout(io.StringIO()):
            result = harness.run_step(ROOT, logs, step, timeout=5)
        self.assertEqual(result['exit_code'], 124)
        self.assertIn('HTTP server is listening', (logs / 'api-timeout.log').read_text())
        with demo.locked(self.directory):
            pass
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with self.assertRaises(OSError):
            opener.open(f'http://127.0.0.1:{port}/healthz', timeout=0.5)
