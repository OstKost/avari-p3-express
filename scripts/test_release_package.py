import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from package_demo_release import image_identities


class ReleasePackageTests(unittest.TestCase):
    def test_classic_and_containerd_identities(self):
        config = json.dumps({'os':'linux','architecture':'amd64','rootfs':{'diff_ids':[]}}).encode()
        config_id = 'sha256:'+hashlib.sha256(config).hexdigest()
        manifest = json.dumps({'schemaVersion':2,'config':{'digest':config_id},'layers':[]}).encode()
        manifest_id = 'sha256:'+hashlib.sha256(manifest).hexdigest()
        with tempfile.TemporaryDirectory() as folder:
            archive = Path(folder)/'images.tar.gz'
            contents = {'manifest.json':json.dumps([{'Config':'blobs/sha256/'+config_id[7:],'RepoTags':['image:release']}]).encode(),'blobs/sha256/'+config_id[7:]:config,'blobs/sha256/'+manifest_id[7:]:manifest}
            with tarfile.open(archive,'w:gz') as tar:
                for name, value in contents.items():
                    item = tarfile.TarInfo(name);item.size=len(value);tar.addfile(item,io.BytesIO(value))
            self.assertEqual(image_identities(archive,'image:release'),(config_id,[manifest_id]))
            with self.assertRaises(ValueError):
                image_identities(archive,'missing:image')

    def test_legacy_classic_archive_config_identity(self):
        config = json.dumps({'os':'linux','architecture':'amd64'}).encode()
        digest = hashlib.sha256(config).hexdigest()
        with tempfile.TemporaryDirectory() as folder:
            archive = Path(folder)/'images.tar.gz'
            contents = {'manifest.json':json.dumps([{'Config':digest+'.json','RepoTags':['image:release']}]).encode(),digest+'.json':config}
            with tarfile.open(archive,'w:gz') as tar:
                for name, value in contents.items():
                    item = tarfile.TarInfo(name);item.size=len(value);tar.addfile(item,io.BytesIO(value))
            self.assertEqual(image_identities(archive,'image:release'),('sha256:'+digest,[]))
