#!/usr/bin/env python3

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.


from pathlib import Path

import argparse
import os
import shutil
import stat


def install_files(source, target, skip=set()):
    for item in os.scandir(source):
        if item.name in skip:
            continue

        target_item = target / item.name

        if item.is_dir():
            target_item.mkdir(mode=0o755, parents=True, exist_ok=True)
            install_files(item.path, target / item.name)
        else:
            st = item.stat()
            if st.st_mode & stat.S_IXUSR:
                target_perms = 0o755
            else:
                target_perms = 0o644

            print(f'{item.path} -> {target_item} ({oct(target_perms)})')
            shutil.copy(item.path, target_item)
            target_item.chmod(target_perms)


def main():
    parser = argparse.ArgumentParser()

    parser.add_argument('outdir', help='The build directory to install', default='out', type=Path)
    parser.add_argument('--destdir', help='The destdir to install into', default='/', type=Path)
    parser.add_argument('--prefix', help='The prefix to install into', default='/usr/local',
                        type=Path)

    args = parser.parse_args()

    install_files(args.outdir / 'install' / 'etc', args.destdir / 'etc')
    install_files(args.outdir / 'install', args.destdir / args.prefix.relative_to('/'),
                  skip={'etc'})


if __name__ == '__main__':
    main()
