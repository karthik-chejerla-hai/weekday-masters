"""Wrong-target releases must stop before a cloud command can run."""
import os
from pathlib import Path
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[2]


class ReleaseGuardTests(unittest.TestCase):
    def run_release(self, **changes):
        names = ['RELEASE_ENV', 'RELEASE_SHA', 'GCP_PROJECT_ID', 'RUNTIME_SA', 'AUTH0_DOMAIN',
                 'AUTH0_CLIENT_ID', 'AUTH0_AUDIENCE', 'ADMIN_EMAIL', 'FIREBASE_API_KEY',
                 'FIREBASE_APP_ID', 'FIREBASE_MESSAGING_SENDER_ID', 'FIREBASE_VAPID_KEY']
        env = {**os.environ, **dict.fromkeys(names, 'test-only')}
        env.update(RELEASE_ENV='dev', RELEASE_SHA='a' * 40, GCP_PROJECT_ID='rally-club-app-dev')
        env.update(changes)
        return subprocess.run(['bash', '.github/scripts/release.sh'], cwd=ROOT, env=env,
                              capture_output=True, text=True)

    def test_dev_cannot_target_production_project(self):
        result = self.run_release(GCP_PROJECT_ID='rally-club-app')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Environment and cloud project do not match', result.stderr)

    def test_branch_name_cannot_replace_immutable_source(self):
        result = self.run_release(RELEASE_SHA='main')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Invalid source commit', result.stderr)

    def test_missing_dev_setting_has_no_production_fallback(self):
        result = self.run_release(AUTH0_CLIENT_ID='')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Missing release setting: AUTH0_CLIENT_ID', result.stderr)

    def test_different_checkout_cannot_use_reviewed_commit_label(self):
        result = self.run_release()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Checkout does not match release', result.stderr)
