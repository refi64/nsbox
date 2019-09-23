# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

import argparse
import os.path
import subprocess
import tarfile


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--source-root')
    parser.add_argument('--prefix')
    parser.add_argument('--out-tar')
    parser.add_argument('--out-dep')

    args = parser.parse_args()
    dep_parent = os.path.dirname(args.out_dep)

    files_process = subprocess.run(['git', 'ls-files', '-oc', '-X', '.gitignore',
                                    args.source_root],
                                   check=True, stdout=subprocess.PIPE, universal_newlines=True)
    files = files_process.stdout.splitlines()

    # XXX: Avoid a weird issue where the out dependency file's parent dirs don't exist yet.
    os.makedirs(os.path.dirname(args.out_dep), exist_ok=True)

    deps = set()

    with tarfile.open(args.out_tar, 'w') as tar:
        for line in files:
            tar.add(line, os.path.join(args.prefix, os.path.relpath(line, args.source_root)))
            deps.add(os.path.relpath(line))

    with open(args.out_dep, 'w') as dep:
        print(f'{args.out_tar}: {" ".join(deps)}', file=dep)


if __name__ == '__main__':
    main()
