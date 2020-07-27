# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Parses the date of the latest git commit to generate the version.

import argparse
import json
import subprocess
import time


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--root')
    parser.add_argument('--branch', choices=['stable', 'edge'])
    parser.add_argument('--override-commit')

    args = parser.parse_args()

    fmt = '%ct'
    if args.branch == 'edge':
        fmt += '.%h'

    git_version_cmd = ['git', '-C', args.root, 'log', '-1', f'--format={fmt}']
    if args.override_commit is not None:
        git_version_cmd.append(args.override_commit)

    version_proc = subprocess.run(git_version_cmd,
                                  stdout=subprocess.PIPE,
                                  check=True,
                                  universal_newlines=True,
                                  cwd=args.root)
    version_proc_parts = version_proc.stdout.strip().split('.')
    assert len(version_proc_parts) in (1, 2), version_proc_parts

    data = {
        'version':
        time.strftime('%y.%m.%d', time.gmtime(int(version_proc_parts[0]))),
        'commit':
        '',
    }

    if args.branch == 'edge':
        data['commit'] = version_proc_parts[1]

        git_rev_list_cmd = [
            'git', 'rev-list', '--count', args.override_commit or 'HEAD'
        ]
        rev_count_proc = subprocess.run(git_rev_list_cmd,
                                        stdout=subprocess.PIPE,
                                        check=True,
                                        universal_newlines=True,
                                        cwd=args.root)
        rev_count = rev_count_proc.stdout.strip()

        data['version'] += f'.{rev_count}'

    print(json.dumps(data))


if __name__ == '__main__':
    main()
