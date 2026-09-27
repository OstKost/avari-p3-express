#!/usr/bin/env python3
"""Checked release bundles, isolated demo volumes and rollback on a Linux Docker VPS."""
import argparse
from contextlib import contextmanager
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import tarfile
import tempfile
import time
import urllib.request

OWNER = 'avari-p3-express/demo-deployment/v1\n'
FILES = {'deploy.py', 'compose.yml', 'gateway.conf', 'release.json', 'images.tar.gz'}
GATEWAY = 'p3express-demo-gateway'
CREDENTIALS = 'p3express_demo_credentials'
LABEL = 'io.avari.demo=true'


def atomic(path, content):
    fd, temporary = tempfile.mkstemp(dir=path.parent, prefix='.write-')
    try:
        with os.fdopen(fd, 'w') as stream:
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        Path(temporary).unlink(missing_ok=True)


def metadata(path):
    data = json.loads((path / 'release.json').read_text())
    if not re.fullmatch(r'v\d+\.\d+\.\d+', data['version']):
        raise ValueError('release must use vMAJOR.MINOR.PATCH')
    if not re.fullmatch(r'[0-9a-f]{40}', data['commit']):
        raise ValueError('invalid commit')
    if not re.fullmatch(r'\d{1,20}-\d{1,5}', data['deployment_id']):
        raise ValueError('invalid deployment ID')
    if not re.fullmatch(r'\d{4}-\d{2}-\d{2}', data['fixture_date']):
        raise ValueError('invalid fixture date')
    for kind in ('api', 'web'):
        if data[f'{kind}_image'] != f'avari-p3-{kind}:' + data['commit']:
            raise ValueError('image tag does not match commit')
        if not re.fullmatch(r'sha256:[0-9a-f]{64}', data[f'{kind}_image_id']):
            raise ValueError('invalid image digest')
    return data


def verify_bundle(path):
    entries = {}
    for line in (path / 'SHA256SUMS').read_text().splitlines():
        digest, name = line.split()
        if name in entries or name not in FILES or not re.fullmatch('[0-9a-f]{64}', digest):
            raise ValueError('invalid bundle checksum list')
        entries[name] = digest
    if set(entries) != FILES:
        raise ValueError('incomplete bundle checksum list')
    for name, digest in entries.items():
        file = path / name
        if file.is_symlink() or not file.is_file():
            raise ValueError('bundle contains nonregular files')
        with file.open('rb') as stream:
            actual = hashlib.file_digest(stream, 'sha256').hexdigest()
        if actual != digest:
            raise ValueError('bundle checksum mismatch: ' + name)
    return metadata(path)


class Deployment:
    def __init__(self, root):
        self.root = root.absolute()
        if self.root.is_symlink():
            raise ValueError('deployment root cannot be a symlink')
        if not self.root.exists():
            self.root.mkdir(mode=0o700, parents=True)
            (self.root / '.owner').write_text(OWNER)
        if (self.root / '.owner').is_symlink() or (self.root / '.owner').read_text() != OWNER:
            raise ValueError('deployment directory is not owned by this application')
        self.root = self.root.resolve()
        for name in ('releases', 'gateway', 'state.json', 'deploy.lock'):
            if (self.root / name).is_symlink():
                raise ValueError('symlink in deployment directory')
        (self.root / 'releases').mkdir(exist_ok=True, mode=0o700)
        (self.root / 'gateway').mkdir(exist_ok=True, mode=0o700)
        self.docker = ['docker'] if os.geteuid() == 0 else ['sudo', '-n', 'docker']
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    @contextmanager
    def lock(self):
        fd = os.open(self.root / 'deploy.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            yield
        finally:
            os.close(fd)

    def command(self, *args, env=None, check=True):
        prefix = self.docker
        if env and os.geteuid() != 0:
            prefix = ['sudo', '-n', 'env', *[key + '=' + value for key, value in env.items()], 'docker']
        result = subprocess.run([*prefix, *args], env={**os.environ, **(env or {})}, capture_output=True, text=True, timeout=600)
        if check and result.returncode:
            raise RuntimeError('Docker command failed: ' + ' '.join(args[:3]) + '\n' + result.stderr[-2000:])
        return result

    def unpack(self, archive):
        with tempfile.TemporaryDirectory(dir=self.root / 'releases', prefix='.stage-') as temporary:
            path = Path(temporary)
            with tarfile.open(archive, 'r:gz') as tar:
                members = tar.getmembers()
                names = [member.name for member in members]
                if set(names) != FILES | {'SHA256SUMS'} or len(names) != len(FILES) + 1:
                    raise ValueError('unexpected release bundle entries')
                for member in members:
                    if not member.isfile() or member.size > 2 * 1024**3:
                        raise ValueError('unsafe release archive member')
                    with tar.extractfile(member) as source, (path / member.name).open('xb') as target:
                        while chunk := source.read(1024**2):
                            target.write(chunk)
            data = verify_bundle(path)
            final = self.root / 'releases' / (data['version'] + '-' + data['deployment_id'])
            if final.exists():
                if final.is_symlink() or (final / 'SHA256SUMS').read_bytes() != (path / 'SHA256SUMS').read_bytes():
                    raise ValueError('deployment ID already exists with different content')
                verify_bundle(final)
            else:
                os.rename(path, final)
            return self.info(final, data)

    def info(self, path, data):
        return {'release': str(path.relative_to(self.root)), 'project': 'p3d_' + data['commit'][:12] + '_' + data['deployment_id'], 'volume': 'p3express_demo_data_' + data['commit'][:12] + '_' + data['deployment_id'], 'version': data['version'], 'commit': data['commit']}

    def release(self, info):
        path = self.root / info['release']
        if path.is_symlink() or path.parent != self.root / 'releases':
            raise ValueError('invalid release state path')
        data = verify_bundle(path)
        expected = self.info(path, data)
        if any(info.get(key) != value for key, value in expected.items()) or info.get('port') not in (14801, 14802):
            raise ValueError('release state does not match verified bundle')
        return path, data

    def state(self):
        file = self.root / 'state.json'
        state = json.loads(file.read_text()) if file.exists() else {'current': None, 'previous': None}
        for info in (state['current'], state['previous']):
            if info:
                self.release(info)
        return state

    def compose(self, info, *args):
        path, data = self.release(info)
        return self.command('compose', '-p', info['project'], '-f', str(path / 'compose.yml'), *args, env={'API_IMAGE': data['api_image_id'], 'WEB_IMAGE': data['web_image_id'], 'DATA_VOLUME': info['volume'], 'SLOT_PORT': str(info['port'])})

    def volume(self, name):
        result = self.command('volume', 'inspect', name, check=False)
        if result.returncode:
            self.command('volume', 'create', '--label', LABEL, name)
            return True
        labels = json.loads(result.stdout)[0].get('Labels') or {}
        if labels.get('io.avari.demo') != 'true':
            raise ValueError('existing volume belongs to another application')
        return False

    def load(self, path, data):
        self.command('load', '-i', str(path / 'images.tar.gz'))
        for kind in ('api', 'web'):
            actual = self.command('image', 'inspect', '--format', '{{.Id}}', data[kind + '_image']).stdout.strip()
            if actual != data[kind + '_image_id']:
                raise ValueError('loaded image digest mismatch')

    def prepare(self, info):
        path, data = self.release(info)
        self.load(path, data)
        self.volume(CREDENTIALS)
        created = self.volume(info['volume'])
        self.command('run', '--rm', '--user', '0', '--entrypoint', 'sh', '-v', CREDENTIALS + ':/app/credentials', '-v', info['volume'] + ':/app/data', data['api_image'], '-c', 'chown 10001:10001 /app/data /app/credentials && chmod 700 /app/data /app/credentials')
        if created:
            result = self.command('run', '--rm', '--entrypoint', '/app/demo', '-v', info['volume'] + ':/app/data', data['api_image'], '--db', '/app/data/p3.db', '--date', data['fixture_date'], '--profile', 'demo')
            fixture = json.loads(result.stdout)
            if fixture['schema_version'] != 2 or fixture['profile'] != 'demo' or len(fixture['projects']) != 3:
                raise ValueError('seeded fixture is incomplete')
            atomic(path / 'fixture.json', json.dumps(fixture))
        elif not (path / 'fixture.json').is_file():
            raise ValueError('existing candidate data has no fixture manifest; refusing to reseed')
        self.compose(info, 'up', '-d', '--wait', '--wait-timeout', '90')

    def health(self, port, info):
        deadline = time.monotonic() + 60
        while True:
            try:
                with self.opener.open(f'http://127.0.0.1:{port}/healthz', timeout=3) as response:
                    healthy = json.load(response).get('status') == 'ok'
                with self.opener.open(f'http://127.0.0.1:{port}/version.json', timeout=3) as response:
                    version = json.load(response)
                if healthy and version.get('version') == info['version'] and version.get('commit') == info['commit']:
                    return
            except (OSError, ValueError):
                pass
            if time.monotonic() >= deadline:
                raise RuntimeError('candidate health/version check failed')
            time.sleep(1)

    def gateway(self, source):
        directory = self.root / 'gateway'
        file = directory / 'gateway.conf'
        if file.exists() and file.read_bytes() != source.read_bytes():
            raise ValueError('gateway config changes need an explicit infrastructure update')
        if not file.exists():
            atomic(file, source.read_text())
        if not (directory / 'upstream.inc').exists():
            atomic(directory / 'upstream.inc', 'return 503;\n')
        result = self.command('container', 'inspect', GATEWAY, check=False)
        if result.returncode:
            self.command('run', '-d', '--name', GATEWAY, '--label', LABEL, '--network', 'host', '--restart', 'unless-stopped', '-v', str(directory) + ':/etc/nginx/conf.d:ro', 'nginx:1.28-alpine')
        else:
            item = json.loads(result.stdout)[0]
            if item['Config'].get('Labels', {}).get('io.avari.demo') != 'true':
                raise ValueError('gateway container belongs to another application')
            if not item['State']['Running']:
                self.command('start', GATEWAY)

    def switch(self, info):
        atomic(self.root / 'gateway' / 'upstream.inc', f'proxy_pass http://127.0.0.1:{info["port"]};\n')
        self.command('exec', GATEWAY, 'nginx', '-t')
        self.command('exec', GATEWAY, 'nginx', '-s', 'reload')
        self.health(14800, info)

    def save(self, current, previous):
        atomic(self.root / 'state.json', json.dumps({'current': current, 'previous': previous}, indent=2) + '\n')

    def activate(self, new, old, old_previous=None, fresh=False):
        old_include = (self.root / 'gateway' / 'upstream.inc').read_text()
        try:
            self.switch(new)
            self.save(new, old)
        except BaseException:
            atomic(self.root / 'gateway' / 'upstream.inc', old_include)
            self.command('exec', GATEWAY, 'nginx', '-t')
            self.command('exec', GATEWAY, 'nginx', '-s', 'reload')
            self.save(old, old_previous)
            self.compose(new, 'down')
            if fresh:
                self.command('volume', 'rm', new['volume'])
            raise
        if old:
            # Containers stop; data and archives remain for explicit rollback.
            self.compose(old, 'down')

    def apply(self, archive):
        state = self.state()
        new = self.unpack(archive)
        old = state['current']
        if old and new['version'] == old['version']:
            if new['commit'] != old['commit']:
                raise ValueError('a published version cannot change its commit')
            self.health(14800, old)
            print('Release already active; demo data preserved')
            return
        new['port'] = 14802 if old and old['port'] == 14801 else 14801
        path, _ = self.release(new)
        try:
            self.prepare(new)
            self.health(new['port'], new)
            self.gateway(path / 'gateway.conf')
        except BaseException:
            self.compose(new, 'down')
            # This project is a fresh isolated candidate, never current/previous.
            if not state['previous'] or new['volume'] != state['previous']['volume']:
                self.command('volume', 'rm', new['volume'], check=False)
            raise
        self.activate(new, old, state['previous'], fresh=not state['previous'] or new['volume'] != state['previous']['volume'])
        print('Active demo: ' + new['version'] + ' ' + new['commit'])

    def rollback(self):
        state = self.state()
        old, previous = state['current'], state['previous']
        if not old or not previous:
            raise ValueError('no previous demo release to roll back to')
        self.compose(previous, 'up', '-d', '--wait', '--wait-timeout', '90')
        self.health(previous['port'], previous)
        self.activate(previous, old, previous)
        print('Rolled back demo: ' + previous['version'])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['apply', 'rollback'])
    parser.add_argument('--bundle', type=Path)
    parser.add_argument('--root', type=Path, default=Path('/opt/avari-p3-express'))
    args = parser.parse_args()
    deploy = Deployment(args.root)
    def interrupted(signum, frame):
        raise KeyboardInterrupt
    signal.signal(signal.SIGTERM, interrupted)
    with deploy.lock():
        if args.action == 'apply':
            if not args.bundle:
                parser.error('--bundle required')
            deploy.apply(args.bundle)
        else:
            deploy.rollback()


if __name__ == '__main__':
    main()
