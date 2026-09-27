#!/usr/bin/env python3
"""Portable config/manifest image identities from a Docker save archive."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import tarfile


def image_identities(archive, image):
    with tarfile.open(archive, 'r:gz') as tar:
        records = json.load(tar.extractfile('manifest.json'))
        matches = [record for record in records if image in (record.get('RepoTags') or [])]
        if len(matches) != 1:
            raise ValueError('expected one Linux amd64 image: ' + image)
        config_bytes = tar.extractfile(matches[0]['Config']).read()
        config = json.loads(config_bytes)
        if config['os'] != 'linux' or config['architecture'] != 'amd64':
            raise ValueError('unsupported release image platform')
        config_id = 'sha256:' + hashlib.sha256(config_bytes).hexdigest()
        manifests = []
        for member in tar.getmembers():
            if not member.isfile() or not member.name.startswith('blobs/sha256/') or member.size > 1024**2:
                continue
            content = tar.extractfile(member).read()
            try:
                manifest = json.loads(content)
            except (ValueError, UnicodeError):
                continue
            if isinstance(manifest, dict) and isinstance(manifest.get('config'), dict) and manifest['config'].get('digest') == config_id:
                manifests.append('sha256:' + hashlib.sha256(content).hexdigest())
        return config_id, sorted(set(manifests))


def main():
    data = {'version': os.environ['GITHUB_REF_NAME'], 'commit': os.environ['GITHUB_SHA'], 'deployment_id': os.environ['GITHUB_RUN_ID']+'-'+os.environ['GITHUB_RUN_ATTEMPT'], 'fixture_date': datetime.date.today().isoformat()}
    for kind in ('api', 'web'):
        image = 'avari-p3-'+kind+':'+data['commit']
        config, manifests = image_identities(Path('bundle/images.tar.gz'), image)
        data[kind+'_image'] = image
        data[kind+'_image_id'] = config
        data[kind+'_manifest_ids'] = manifests
    Path('bundle/release.json').write_text(json.dumps(data))


if __name__ == '__main__':
    main()
