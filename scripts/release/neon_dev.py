#!/usr/bin/env python3
"""Store an existing, separate Neon dev project's URI using its project-scoped key."""
import json
import os
import re
import subprocess
import urllib.error
import urllib.parse
import urllib.request

PROJECT = 'rally-club-app-dev'
PRODUCTION_NEON_PROJECT = 'spring-rain-30756651'


def neon(path):
    request = urllib.request.Request('https://console.neon.tech/api/v2/' + path,
                                     headers={'Authorization': 'Bearer ' + os.environ['DEV_NEON_API_KEY']})
    try:
        with urllib.request.urlopen(request, timeout=90) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        try:
            message = json.loads(error.read()).get('message', 'No error detail')
        except (ValueError, AttributeError):
            message = 'No JSON error detail'
        message = message.replace(os.environ['DEV_NEON_API_KEY'], '[redacted]')
        message = re.sub(r'postgres(?:ql)?://\S+', '[redacted]', message)
        raise RuntimeError(f'Neon HTTP {error.code}: {message[:300]}') from None


def store(name, value):
    found = subprocess.run(['gcloud', 'secrets', 'describe', name, '--project=' + PROJECT],
                           capture_output=True, text=True)
    if found.returncode:
        if 'NOT_FOUND' not in found.stderr:
            raise RuntimeError('Cannot inspect dev secret')
        subprocess.run(['gcloud', 'secrets', 'create', name, '--project=' + PROJECT,
                        '--replication-policy=automatic'], check=True)
    subprocess.run(['gcloud', 'secrets', 'versions', 'add', name, '--project=' + PROJECT,
                    '--data-file=-'], input=value, text=True, check=True)


def main():
    project_id = os.environ.get('DEV_NEON_PROJECT_ID', '')
    if not project_id or project_id == PRODUCTION_NEON_PROJECT:
        raise RuntimeError('Set DEV_NEON_PROJECT_ID to a separate dev project. Production is forbidden.')
    saved = subprocess.run(['gcloud', 'secrets', 'versions', 'access', 'latest',
                            '--secret=dev-neon-project', '--project=' + PROJECT],
                           capture_output=True, text=True)
    if saved.returncode == 0:
        identity = json.loads(saved.stdout)
        if identity['project_id'] != project_id:
            raise RuntimeError('Saved dev database differs from configured project. Review before changing it.')
        ready = subprocess.run(['gcloud', 'secrets', 'versions', 'access', 'latest',
                                '--secret=dev-database-url', '--project=' + PROJECT], capture_output=True)
        if ready.returncode:
            raise RuntimeError('Dev database identity exists but its credential is unavailable')
        print('Dev database already configured: ' + project_id)
        return
    if 'NOT_FOUND' not in saved.stderr:
        raise RuntimeError('Cannot inspect dev database identity')
    project = neon('projects/' + project_id)['project']
    if project['id'] != project_id or project['name'] != 'Rally Dev':
        raise RuntimeError('The configured Neon project must be the separate Rally Dev project')
    database = os.environ.get('DEV_NEON_DB_NAME', 'neondb')
    role = os.environ.get('DEV_NEON_DB_ROLE', 'neondb_owner')
    query = urllib.parse.urlencode({'database_name': database, 'role_name': role})
    uri = neon('projects/' + project_id + '/connection_uri?' + query)['uri']
    store('dev-database-url', uri)
    store('dev-neon-project', json.dumps({'project_id': project_id, 'database': database, 'role': role}))
    print('Separate dev database configured: ' + project_id)


if __name__ == '__main__':
    main()
