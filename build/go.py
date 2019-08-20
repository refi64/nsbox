# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Calls the go compiler, but also generates a Makefile-style .d file containing the Go package's
# dependencies, that way GN can track them.

from pathlib import Path

import argparse
import json
import os
import subprocess
import sys


def load_deps(args):
    process = subprocess.Popen([args.go, 'list', '-mod=vendor', '-json', '-deps', args.package],
                               stdout=subprocess.PIPE, universal_newlines=True)
    dep_json, _ = process.communicate()

    if process.returncode:
        return

    # Change formatting to be in list form.
    dep_json = '[' + dep_json.replace('\n}', '\n},').rstrip(',\n') + ']'

    deps = json.loads(dep_json)

    with open(args.out_dep, 'w') as fp:
        for dep in deps:
            import_path = dep['ImportPath']
            directory = Path(dep['Dir']).resolve()

            try:
                directory.relative_to(args.root)
            except ValueError:
                continue

            for key, value in dep.items():
                if key.endswith('Files') and not key.startswith('Ignored'):
                    for f in value:
                        print(f'{args.out_bin}:', ' '.join(str(directory / f) for f in value),
                              file=fp)


def build(args):
    process = subprocess.Popen([args.go, 'build', '-mod=vendor', '-o', args.out_bin, args.package])
    ret = process.wait()
    if ret:
        sys.exit(ret)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--go')
    parser.add_argument('--root', type=lambda x: Path(x).resolve())
    parser.add_argument('--package')
    parser.add_argument('--out-bin')
    parser.add_argument('--out-dep')

    args = parser.parse_args()

    os.chdir(args.root)
    load_deps(args)
    build(args)


if __name__ == '__main__':
    main()
