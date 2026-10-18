import json
import os
import subprocess
import unittest
from unittest.mock import patch
import neon_dev


class DevDatabaseTests(unittest.TestCase):
    @patch('neon_dev.subprocess.run')
    @patch('neon_dev.neon')
    def test_existing_secret_prevents_any_neon_project_creation(self, api, run):
        run.side_effect = [subprocess.CompletedProcess([], 0, json.dumps({'project_id': 'dev-id'}), ''),
                           subprocess.CompletedProcess([], 0, b'credential', b'')]
        neon_dev.main()
        api.assert_not_called()

    @patch('neon_dev.store')
    @patch('neon_dev.subprocess.run')
    @patch('neon_dev.neon')
    def test_new_project_has_no_production_parent_branch(self, api, run, store):
        run.return_value = subprocess.CompletedProcess([], 1, '', 'NOT_FOUND')
        api.side_effect = [{'project': {'region_id': 'aws-ap-southeast-2', 'org_id': 'org'}},
                           {'projects': []},
                           {'project': {'id': 'new-dev'}, 'connection_uris': [{'connection_uri': 'dev-only'}]}]
        with patch.dict(os.environ, {'NEON_REFERENCE_PROJECT_ID': 'prod-metadata-only'}):
            neon_dev.main()
        create = api.call_args_list[-1]
        self.assertEqual(create.args[0:2], ('projects', 'POST'))
        config = create.args[2]['project']
        self.assertNotIn('parent_id', config['branch'])
        self.assertEqual(config['branch']['database_name'], 'rally_dev')
        store.assert_any_call('dev-database-url', 'dev-only')

    @patch('neon_dev.subprocess.run')
    @patch('neon_dev.neon')
    def test_permission_error_cannot_create_duplicate_database(self, api, run):
        run.return_value = subprocess.CompletedProcess([], 1, '', 'PERMISSION_DENIED')
        with self.assertRaises(RuntimeError):
            neon_dev.main()
        api.assert_not_called()

    @patch('neon_dev.subprocess.run')
    @patch('neon_dev.neon')
    def test_missing_credential_with_saved_identity_stops(self, api, run):
        run.side_effect = [subprocess.CompletedProcess([], 0, json.dumps({'project_id': 'dev-id'}), ''),
                           subprocess.CompletedProcess([], 1, b'', b'NOT_FOUND')]
        with self.assertRaises(RuntimeError):
            neon_dev.main()
        api.assert_not_called()
