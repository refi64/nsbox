# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Writes the VERSION and RELEASE files for use by other targets.

import argparse
import time


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--version')
    parser.add_argument('--branch')
    parser.add_argument('--out-version')
    parser.add_argument('--out-branch')

    args = parser.parse_args()

    with open(args.out_version, 'w') as fp:
        fp.write(args.version)

    with open(args.out_branch, 'w') as fp:
        fp.write(args.branch)


if __name__ == '__main__':
    main()
