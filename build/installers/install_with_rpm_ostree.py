#!/usr/bin/env python3

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.


from pathlib import Path
import argparse
import json
import os
import re
import subprocess
import sys


def rpm_ostree(args):
    prefix = ['flatpak-spawn', '--host'] if os.path.exists('/run/host/nsbox') else []
    return [*prefix, 'rpm-ostree', *args]


def read_fedora_release():
    with open('/etc/os-release') as fp:
        for line in fp:
            if line.startswith('VERSION_ID='):
                return int(line.split('=', 1)[1])

    assert False


def get_matching_rpms(args):
    build_dir = Path(__file__).parent.parent
    rpm_dir = args.outdir / 'rpm'

    release_process = subprocess.run([sys.executable, str(build_dir / 'parse_release.py'),
                                      f'--root={build_dir.parent}', f'--branch={args.branch}'],
                                      stdout=subprocess.PIPE, check=True, universal_newlines=True)
    release = json.loads(release_process.stdout)
    release_version = args.version or release['version']

    fedora = read_fedora_release()

    rpms = []

    for child in rpm_dir.iterdir():
        name = child.name
        keep = (
            not ('debug' in name or 'guest-tools' in name
                    or 'bender' in name or 'alias' in name
                    or name.endswith('.src.rpm'))
            and release_version in name
            and (not name.startswith('nsbox-edge-') if args.branch == 'stable'
                    else release['commit'] in name)
        )
        if not keep:
            continue

        rpms.append(child)

    return rpms


def get_packages_to_uninstall(args):
    rpm_ostree_state_process = subprocess.run(rpm_ostree(['status', '--json']),
                                              stdout=subprocess.PIPE, check=True,
                                              universal_newlines=True)
    rpm_ostree_state = json.loads(rpm_ostree_state_process.stdout)

    booted_deloyment = [deployment for deployment in rpm_ostree_state['deployments'] if deployment['booted']][0]

    if args.branch == 'stable':
        package_re = re.compile(r'(nsbox(?!-edge)(?:-[a-z]+)*)')
    else:
        package_re = re.compile(r'(nsbox-edge(?:-[a-z]+)*)')

    requested = booted_deloyment['requested-packages'] + booted_deloyment['requested-local-packages']
    to_uninstall = []

    for package in requested:
        match = package_re.match(package)
        if match is None:
            continue

        to_uninstall.append(package)

    return to_uninstall


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('outdir', help='The build directory to install', default='out', type=Path)
    parser.add_argument('--branch', help='The release branch', default='edge', choices=['edge', 'stable'])
    parser.add_argument('--version', help='The release version')
    args = parser.parse_args()

    rpms = get_matching_rpms(args)
    to_uninstall = get_packages_to_uninstall(args)

    args = ['install']
    args.extend(f'--uninstall={package}' for package in to_uninstall)
    args.extend(map(str, rpms))

    subprocess.run(rpm_ostree(args), check=True)


if __name__ == '__main__':
    main()
