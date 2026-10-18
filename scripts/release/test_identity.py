import os
import unittest
from unittest.mock import patch
from identity import validate


class ReleaseIdentityTests(unittest.TestCase):
    def setUp(self):
        self.release = {'environment': 'dev', 'sha': 'a' * 40, 'digest': 'sha256:abc', 'image': 'dev/image'}
        self.env = patch.dict(os.environ, {'RELEASE_SHA': 'a' * 40})
        self.env.start()
        self.addCleanup(self.env.stop)

    def test_same_release_can_be_promoted(self):
        validate(self.release, dict(self.release))

    def test_new_dev_deploy_blocks_old_approval(self):
        for field, value in [('sha', 'b' * 40), ('digest', 'sha256:other'), ('environment', 'prod')]:
            with self.subTest(field=field), self.assertRaises(ValueError):
                validate(self.release, dict(self.release, **{field: value}))

    def test_artifact_cannot_select_another_source(self):
        with patch.dict(os.environ, {'RELEASE_SHA': 'b' * 40}), self.assertRaises(ValueError):
            validate(self.release, self.release)

    def test_production_manifest_cannot_act_as_dev_approval(self):
        release = dict(self.release, environment='prod')
        with self.assertRaises(ValueError):
            validate(release, release)
