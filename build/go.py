# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Calls the go compiler, but also generates a Makefile-style .d file containing the Go package's
# dependencies, that way GN can track them.

from go_deps import go_list

from pathlib import Path

import argparse
import os
import subprocess
import sys


def load_deps(args, tags):
    deps = go_list(args.go, args.package, cwd=str(args.gofiles_root), vendor=True,
                   tags=tags)

    bin_relative_to_depfile = os.path.relpath(args.out_bin, args.out_dep)

    makefile_deps = set()

    for dep in deps:
        import_path = dep['ImportPath']
        directory = Path(dep['Dir']).resolve()

        try:
            directory.relative_to(args.gofiles_root)
        except ValueError:
            continue

        for key, value in dep.items():
            if key.endswith('Files') and not key.startswith('Ignored'):
                makefile_deps |= set((directory / f).relative_to(Path().resolve()) for f in value)

    with open(args.out_dep, 'w') as fp:
        print(f'{args.out_bin}: {" ".join(map(str, makefile_deps))}', file=fp)


def build(args, tags):
    command = [args.go, 'build', '-mod=vendor', '-o', os.path.abspath(args.out_bin)]
    env = os.environ.copy()

    env['GOCACHE'] = args.go_cache

    if args.static:
        env['CGO_ENABLED'] = '0'
        # We have to add buildmode=exe because distros like to use buildmode=pie for
        # building Go binaries, but it causes statically-linked binaries like nsbox-host
        # to segfault.
        command.extend(['-ldflags', '-extldflags "-static"', '-buildmode=exe'])

    if tags:
        command.append(f'-tags={" ".join(tags)}')

    command.append(args.package)

    process = subprocess.Popen(command, cwd=str(args.gofiles_root), env=env)
    ret = process.wait()
    if ret:
        sys.exit(ret)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--go')
    parser.add_argument('--go-cache')
    parser.add_argument('--gofiles-root', type=lambda x: Path(x).resolve())
    parser.add_argument('--package')
    parser.add_argument('--out-bin')
    parser.add_argument('--out-dep')
    parser.add_argument('--selinux', action='store_true', default=False)
    parser.add_argument('--static', action='store_true', default=False)

    args = parser.parse_args()

    tags = set()
    if args.selinux:
        tags.add('selinux')

    load_deps(args, tags)
    build(args, tags)


if __name__ == '__main__':
    main()
