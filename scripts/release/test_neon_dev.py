import json
import os
import subprocess
import unittest
from unittest.mock import patch
import neon_dev


class DevDatabaseTests(unittest.TestCase):
    def setUp(self):
        env = patch.dict(os.environ, {'DEV_NEON_PROJECT_ID': 'dev-id'})
        env.start()
        self.addCleanup(env.stop)

    @patch('neon_dev.subprocess.run')
    @patch('neon_dev.neon')
    def test_existing_secret_needs_no_neon_administration_key(self, api, run):
        run.side_effect = [subprocess.CompletedProcess([], 0, json.dumps({'project_id': 'dev-id'}), ''),
                           subprocess.CompletedProcess([], 0, b'credential', b'')]
        neon_dev.main()
        api.assert_not_called()

    @patch('neon_dev.store')
    @patch('neon_dev.subprocess.run')
    @patch('neon_dev.neon')
    def test_only_separate_dev_project_is_read(self, api, run, store):
        run.return_value = subprocess.CompletedProcess([], 1, '', 'NOT_FOUND')
        api.side_effect = [{'project': {'id': 'dev-id', 'name': 'Rally Dev'}}, {'uri': 'dev-only'}]
        neon_dev.main()
        self.assertTrue(all(call.args[0].startswith('projects/dev-id') for call in api.call_args_list))
        store.assert_any_call('dev-database-url', 'dev-only')

    @patch('neon_dev.subprocess.run')
    @patch('neon_dev.neon')
    def test_production_project_is_rejected_before_any_cloud_access(self, api, run):
        with patch.dict(os.environ, {'DEV_NEON_PROJECT_ID': neon_dev.PRODUCTION_NEON_PROJECT}), self.assertRaises(RuntimeError):
            neon_dev.main()
        api.assert_not_called()
        run.assert_not_called()

    @patch('neon_dev.subprocess.run')
    @patch('neon_dev.neon')
    def test_permission_error_cannot_replace_database(self, api, run):
        run.return_value = subprocess.CompletedProcess([], 1, '', 'PERMISSION_DENIED')
        with self.assertRaises(RuntimeError):
            neon_dev.main()
        api.assert_not_called()

    @patch('neon_dev.subprocess.run')
    def test_project_change_requires_review(self, run):
        run.return_value = subprocess.CompletedProcess([], 0, json.dumps({'project_id': 'another-project'}), '')
        with self.assertRaises(RuntimeError):
            neon_dev.main()

    @patch('neon_dev.subprocess.run')
    @patch('neon_dev.neon')
    def test_missing_credential_with_saved_identity_stops(self, api, run):
        run.side_effect = [subprocess.CompletedProcess([], 0, json.dumps({'project_id': 'dev-id'}), ''),
                           subprocess.CompletedProcess([], 1, b'', b'NOT_FOUND')]
        with self.assertRaises(RuntimeError):
            neon_dev.main()
        api.assert_not_called()
