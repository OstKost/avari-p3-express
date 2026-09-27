#!/usr/bin/env python3
"""Owned demo generations, atomic publication and supervised local API (POSIX)."""
import argparse
from contextlib import closing, contextmanager
import fcntl
import json
import os
from pathlib import Path
import re
import signal
import sqlite3
import subprocess
import sys
import tempfile
import uuid

ROOT = Path(__file__).resolve().parents[1]
MARKER = 'avari-demo-v1\n'
VERSION = 2
OWNED_FILES = ('.owner', 'demo.db', 'demo.db-wal', 'demo.db-shm', 'manifest.json')


def sync_directory(directory):
    fd = os.open(directory, os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def write_json(path, value):
    # Same-filesystem temporary file; readers see one complete JSON document.
    fd, temporary = tempfile.mkstemp(prefix='.publish-', dir=path.parent)
    try:
        with os.fdopen(fd, 'w') as stream:
            json.dump(value, stream, ensure_ascii=False, indent=2)
            stream.write('\n')
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        sync_directory(path.parent)
    finally:
        Path(temporary).unlink(missing_ok=True)


def guard(path):
    if path.is_symlink():
        raise ValueError(f'symlink is not supported: {path.name}')


def owned(directory, create=False):
    guard(directory)
    if not directory.exists() and create:
        directory.mkdir(parents=True, mode=0o700)
        (directory / '.owner').write_text(MARKER)
        sync_directory(directory)
    marker = directory / '.owner'
    guard(marker)
    if not marker.is_file() or marker.read_text() != MARKER:
        raise ValueError('directory is not owned by the demo tool; refusing to change it')
    for name in ('.lock', 'current.json', 'generations', 'manager.key', 'server', *OWNED_FILES):
        guard(directory / name)
    return directory.resolve()


@contextmanager
def locked(directory, create=False):
    directory = owned(directory, create)
    fd = os.open(directory / '.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise ValueError('demo API or another demo command is running; stop it first') from error
        yield directory, fd
    finally:
        os.close(fd)


def generation_path(directory, generation):
    if not isinstance(generation, str) or not re.fullmatch(r'[0-9a-f]{32}', generation):
        raise ValueError('invalid generation identifier')
    path = directory / 'generations' / generation
    owned(path)
    return path


def validate(path):
    for name in OWNED_FILES:
        guard(path / name)
    manifest = json.loads((path / 'manifest.json').read_text())
    if manifest.get('schema_version') != VERSION or manifest.get('fixture_version') != 1:
        raise ValueError('unsupported fixture version; run reset explicitly')
    db_path = path / 'demo.db'
    with closing(sqlite3.connect(db_path.as_uri() + '?mode=ro', uri=True, timeout=5)) as db:
        if db.execute('PRAGMA quick_check').fetchall() != [('ok',)]:
            raise ValueError('demo database failed integrity check')
        if db.execute('PRAGMA foreign_key_check').fetchall():
            raise ValueError('demo database has broken foreign keys')
        recorded = json.loads(db.execute('SELECT manifest FROM demo_fixture WHERE id=1').fetchone()[0])
        if recorded != manifest:
            raise ValueError('database and manifest disagree; run reset explicitly')
        for table, key in (('p3_projects', 'projects'), ('work_tasks', 'tasks'), ('p3_steps', 'steps')):
            ids = {row[0] for row in db.execute(f'SELECT id FROM {table}')}
            if not set(manifest[key].values()) <= ids:
                raise ValueError(f'manifest references missing {key}')
    return manifest


def current(directory):
    pointer = directory / 'current.json'
    if not pointer.exists():
        if (directory / 'demo.db').exists():
            raise ValueError('legacy demo layout; run reset explicitly (legacy data is retained until clean)')
        raise ValueError('run seed first')
    data = json.loads(pointer.read_text())
    if data.get('schema_version') != VERSION:
        raise ValueError('unsupported pointer version')
    path = generation_path(directory, data['generation'])
    manifest = validate(path)
    return path, manifest


def run(command, lock_fd, timeout=120, on_start=None, **kwargs):
    """A killed wrapper cannot release ownership while a child still uses data."""
    process = subprocess.Popen(command, pass_fds=(lock_fd,), start_new_session=True, **kwargs)
    if on_start:
        on_start(process.pid)
    def stop(signum, frame):
        try:
            os.killpg(process.pid, signum)
        except ProcessLookupError:
            pass
    previous = {sig: signal.signal(sig, stop) for sig in (signal.SIGINT, signal.SIGTERM)}
    try:
        try:
            stdout, stderr = process.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.communicate()
            raise TimeoutError(f'command timed out after {timeout}s: {command[0]}') from None
        if process.returncode:
            # Go generator/build output has no credentials. API output stays on console.
            detail = stderr or stdout or ''
            raise RuntimeError(f'command failed ({process.returncode}): {command[0]}\n{detail}')
        return stdout
    finally:
        if process.poll() is None:
            stop(signal.SIGTERM, None)
            try:
                process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                stop(signal.SIGKILL, None)
                process.wait()
        for sig, handler in previous.items():
            signal.signal(sig, handler)


def stage(directory):
    generations = directory / 'generations'
    guard(generations)
    if not generations.exists():
        generations.mkdir(mode=0o700)
        (generations / '.owner').write_text(MARKER)
    owned(generations)
    path = generations / uuid.uuid4().hex
    owned(path, create=True)
    return path


def publish(directory, path):
    validate(path)
    for name in ('demo.db', 'manifest.json', '.owner'):
        with (path / name).open('rb') as stream:
            os.fsync(stream.fileno())
    sync_directory(path)
    sync_directory(path.parent)
    write_json(directory / 'current.json', {'schema_version': VERSION, 'generation': path.name})


def discard_unpublished(directory, path):
    pointer = directory / 'current.json'
    if pointer.exists():
        try:
            if json.loads(pointer.read_text()).get('generation') == path.name:
                return
        except (ValueError, AttributeError):
            pass
    remove_generation(path)


def remove_generation(path):
    owned(path)
    for name in OWNED_FILES:
        guard(path / name)
    for name in OWNED_FILES:
        if name != '.owner':
            (path / name).unlink(missing_ok=True)
    # Preserve unrelated files and their ownership marker.
    if set(item.name for item in path.iterdir()) == {'.owner'}:
        (path / '.owner').unlink()
        path.rmdir()


def clean(directory):
    generations = directory / 'generations'
    if generations.exists():
        owned(generations)
        paths = [generation_path(directory, item.name) for item in generations.iterdir()
                 if re.fullmatch(r'[0-9a-f]{32}', item.name)]
        # Validate every deletion target before removing the active pointer.
        for path in paths:
            for name in OWNED_FILES:
                guard(path / name)
        (directory / 'current.json').unlink(missing_ok=True)
        sync_directory(directory)
        for path in paths:
            remove_generation(path)
    for name in ('demo.db', 'demo.db-wal', 'demo.db-shm', 'manifest.json'):
        (directory / name).unlink(missing_ok=True)


def copy_generation(directory, source, manifest):
    path = stage(directory)
    try:
        # SQLite backup includes committed WAL pages; no raw file copy.
        with closing(sqlite3.connect((source / 'demo.db').as_uri() + '?mode=ro', uri=True)) as db:
            with closing(sqlite3.connect(path / 'demo.db')) as target:
                db.backup(target)
        os.chmod(path / 'demo.db', 0o600)
        write_json(path / 'manifest.json', manifest)
        publish(directory, path)
    except BaseException:
        discard_unpublished(directory, path)
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=['seed', 'reset', 'clean', 'api', 'paths', 'backup', 'restore'])
    parser.add_argument('--dir', type=Path, default=ROOT / '.demo')
    parser.add_argument('--date', default='2026-09-27')
    parser.add_argument('--profile', choices=['demo', 'smoke'], default='demo')
    parser.add_argument('--to', type=Path, help='new owned backup directory')
    parser.add_argument('--from', dest='source', type=Path, help='owned backup directory')
    args = parser.parse_args()
    try:
        with locked(args.dir.absolute(), create=args.command in ('seed', 'reset', 'clean', 'restore')) as (directory, lock_fd):
            if args.command == 'clean':
                clean(directory)
                print('Demo data removed; manager key retained')
                return
            if args.command == 'seed' and ((directory / 'current.json').exists() or (directory / 'demo.db').exists()):
                path, manifest = current(directory)
                if manifest['profile'] != args.profile or manifest['reference_date'] != args.date:
                    raise ValueError('existing fixture has another profile/date; run reset explicitly')
                print(f'Demo already exists: {path}; use reset to recreate it')
                return
            if args.command in ('seed', 'reset'):
                path = stage(directory)
                try:
                    output = run(['go', 'run', './cmd/demo', '--db', str(path / 'demo.db'), '--date', args.date, '--profile', args.profile], lock_fd, cwd=ROOT / 'apps/api', stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
                    manifest = json.loads(output)
                    with closing(sqlite3.connect(path / 'demo.db')) as db:
                        db.execute('CREATE TABLE demo_fixture (id INTEGER PRIMARY KEY CHECK(id=1), manifest TEXT NOT NULL)')
                        db.execute('INSERT INTO demo_fixture VALUES (1, ?)', (json.dumps(manifest, ensure_ascii=False),))
                        db.commit()
                        db.execute('PRAGMA wal_checkpoint(TRUNCATE)')
                    write_json(path / 'manifest.json', manifest)
                    publish(directory, path)
                except BaseException:
                    discard_unpublished(directory, path)
                    raise
                print(f'Demo ready: {path}; use paths for machine-readable filenames')
                return
            if args.command == 'restore':
                if not args.source:
                    raise ValueError('--from required')
                with locked(args.source.absolute()) as (source, _):
                    source_path, manifest = current(source)
                    copy_generation(directory, source_path, manifest)
                print('Demo restored; manager key retained')
                return
            path, manifest = current(directory)
            if args.command == 'paths':
                print(json.dumps({'db': str(path / 'demo.db'), 'manifest': str(path / 'manifest.json'), 'manager_key': str(directory / 'manager.key')}))
            elif args.command == 'backup':
                if not args.to:
                    raise ValueError('--to required')
                with locked(args.to.absolute(), create=True) as (destination, _):
                    if (destination / 'current.json').exists() or (destination / 'demo.db').exists():
                        raise ValueError('backup destination already contains data')
                    copy_generation(destination, path, manifest)
                print('Consistent SQLite backup created (no credentials copied)')
            elif args.command == 'api':
                binary = directory / 'server'
                run(['go', 'build', '-o', str(binary), './cmd/server'], lock_fd, cwd=ROOT / 'apps/api')
                env = dict(os.environ, DB_PATH=str(path / 'demo.db'), MANAGER_KEY_PATH=str(directory / 'manager.key'), HOST='127.0.0.1')
                print(f'Demo API; manager key: {directory / "manager.key"}', flush=True)
                run([str(binary)], lock_fd, timeout=None, cwd=ROOT / 'apps/api', env=env, on_start=lambda pid: print(f"Demo API PID: {pid}", flush=True))
    except (OSError, ValueError, RuntimeError, sqlite3.Error, KeyError, TypeError, AttributeError) as error:
        parser.exit(1, f'demo: {error}\n')


if __name__ == '__main__':
    main()
