# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

from pathlib import Path

import argparse
import bz2
import os
import shutil
import subprocess


def run_make(args, *targets):
    subprocess.run([args.make, f'NAME={args.variant}', '-f', args.makefile, *targets],
                   cwd=args.scratch_dir, check=True, stdout=subprocess.DEVNULL)


def compress(args):
    policy_file = os.path.join(args.scratch_dir,
                               os.path.splitext(os.path.basename(args.out))[0])

    with open(policy_file, 'rb') as policy, open(args.out, 'wb') as out:
        compressor = bz2.BZ2Compressor()

        while True:
            data = policy.read(2048)
            if not data:
                break

            out.write(compressor.compress(data))

        out.write(compressor.flush())


def main():
    parser = argparse.ArgumentParser()

    parser.add_argument('--make')
    parser.add_argument('--makefile')
    parser.add_argument('--variant')
    parser.add_argument('--out')
    parser.add_argument('--scratch-dir')
    parser.add_argument('--te')
    parser.add_argument('--fc')

    args = parser.parse_args()

    try:
        shutil.rmtree(args.scratch_dir)
    except FileNotFoundError:
        pass

    os.makedirs(args.scratch_dir, exist_ok=True)

    try:
        shutil.copy(args.te, args.scratch_dir)
        if args.fc:
            shutil.copy(args.fc, args.scratch_dir)

        run_make(args)
        compress(args)
        run_make(args, 'clean')

    finally:
        shutil.rmtree(args.scratch_dir)


if __name__ == '__main__':
    main()
