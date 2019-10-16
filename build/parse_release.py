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

    args = parser.parse_args()

    fmt = '%ct'
    if args.branch == 'edge':
        fmt += '.%h'

    version_proc = subprocess.run(['git', '-C', args.root, 'log', '-1', f'--format={fmt}'],
                                  stdout=subprocess.PIPE, check=True, universal_newlines=True)
    version_proc_parts = version_proc.stdout.strip().split('.')
    assert len(version_proc_parts) in (1, 2), version_proc_parts

    version_proc_parts[0] = time.strftime('%y.%m.%d', time.gmtime(int(version_proc_parts[0])))

    print('.'.join(version_proc_parts), end='')

if __name__ == '__main__':
    main()
