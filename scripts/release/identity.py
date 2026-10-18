"""Fail closed when the reviewed dev release has changed."""
import json
import os
import sys
import urllib.request


def read_live(url):
    request = urllib.request.Request(url, headers={'Cache-Control': 'no-cache'})
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


def validate(expected, actual):
    if expected != actual:
        raise ValueError('Dev release changed. Deploy and test the intended commit again.')
    if expected['sha'] != os.environ['RELEASE_SHA'] or expected['environment'] != 'dev':
        raise ValueError('Release artifact does not identify the approved dev commit.')


def main():
    command, path = sys.argv[1:]
    if command == 'write':
        release = dict(environment=os.environ['RELEASE_ENV'], sha=os.environ['RELEASE_SHA'],
                       image=os.environ['RELEASE_IMAGE'], digest=os.environ['RELEASE_DIGEST'])
        with open(path, 'w') as file:
            json.dump(release, file)
    elif command == 'verify':
        with open(path) as file:
            expected = json.load(file)
        validate(expected, read_live('https://rally-club-app-dev.web.app/release.json'))
    elif command == 'live':
        with open('/tmp/release.json') as file:
            expected = json.load(file)
        if read_live(path) != expected:
            raise ValueError('Live frontend does not match the deployed release.')
    else:
        raise ValueError('Unknown release operation')


if __name__ == '__main__':
    main()
