#!/usr/bin/env python3
"""Provision a fresh dev Neon project. Never branch or read production data."""
import json
import os
import subprocess
import urllib.error
import urllib.request
import urllib.parse

PROJECT = 'rally-club-app-dev'
NAME = 'Rally Dev'


def neon(path, method='GET', body=None):
    request = urllib.request.Request(
        'https://console.neon.tech/api/v2/' + path,
        data=json.dumps(body).encode() if body is not None else None,
        headers={'Authorization': 'Bearer ' + os.environ['NEON_API_KEY'], 'Content-Type': 'application/json'},
        method=method,
    )
    try:
        with urllib.request.urlopen(request, timeout=90) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        raise RuntimeError(f'Neon request failed with HTTP {error.code}; no credential payload logged') from None


def store(name, value):
    command = ['gcloud', 'secrets', 'describe', name, '--project=' + PROJECT, '--format=value(name)']
    found = subprocess.run(command, capture_output=True, text=True)
    if found.returncode:
        if 'NOT_FOUND' not in found.stderr:
            raise RuntimeError('Cannot inspect dev secret')
        subprocess.run(['gcloud', 'secrets', 'create', name, '--project=' + PROJECT,
                        '--replication-policy=automatic'], check=True)
    subprocess.run(['gcloud', 'secrets', 'versions', 'add', name, '--project=' + PROJECT,
                    '--data-file=-'], input=value, text=True, check=True)


def main():
    # Recover by project identity before creating anything, including after an
    # interrupted run that created Neon but did not save the connection URI.
    saved = subprocess.run(['gcloud', 'secrets', 'versions', 'access', 'latest',
                            '--secret=dev-neon-project', '--project=' + PROJECT],
                           capture_output=True, text=True)
    if saved.returncode == 0:
        identity = json.loads(saved.stdout)
        # Require both the identity and credential before considering provisioning complete.
        ready = subprocess.run(['gcloud', 'secrets', 'versions', 'access', 'latest',
                                '--secret=dev-database-url', '--project=' + PROJECT],
                               capture_output=True)
        if ready.returncode:
            raise RuntimeError('Dev database identity exists but its credential is unavailable')
        print('Dev database already provisioned: ' + identity['project_id'])
        return
    if 'NOT_FOUND' not in saved.stderr:
        raise RuntimeError('Cannot inspect dev database identity')
    parent = neon('projects/' + os.environ['NEON_REFERENCE_PROJECT_ID'])['project']
    org = parent.get('org_id')
    suffix = '?org_id=' + org if org else ''
    projects = []
    cursor = ''
    while True:
        page = neon('projects' + suffix + ('&' if suffix else '?') + 'limit=100' +
                    ('&cursor=' + urllib.parse.quote(cursor) if cursor else ''))
        projects.extend(page.get('projects', []))
        next_cursor = page.get('pagination', {}).get('cursor')
        if not next_cursor or next_cursor == cursor or len(page.get('projects', [])) < 100:
            break
        cursor = next_cursor
    matches = [p for p in projects if p['name'] == NAME]
    if len(matches) > 1:
        raise RuntimeError('Multiple Rally Dev projects found; select the intended project before retrying')
    if matches:
        project = matches[0]
        uri = neon('projects/' + project['id'] + '/connection_uri?database_name=rally_dev&role_name=rally_dev')['uri']
    else:
        settings = {'name': NAME, 'region_id': parent['region_id'], 'pg_version': 16,
                    'branch': {'name': 'main', 'database_name': 'rally_dev', 'role_name': 'rally_dev'},
                    'default_endpoint_settings': {'autoscaling_limit_min_cu': 0.25,
                                                  'autoscaling_limit_max_cu': 0.25,
                                                  'suspend_timeout_seconds': 300}}
        if org:
            settings['org_id'] = org
        result = neon('projects', 'POST', {'project': settings})
        project = result['project']
        uri = result['connection_uris'][0]['connection_uri']
    store('dev-database-url', uri)
    store('dev-neon-project', json.dumps({'project_id': project['id'], 'database': 'rally_dev', 'role': 'rally_dev'}))
    print('Separate empty Neon dev project ready: ' + project['id'])


if __name__ == '__main__':
    main()
