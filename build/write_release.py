# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Writes the VERSION and RELEASE files for use by other targets.

import argparse
import time


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--head-file')
    parser.add_argument('--branch')
    parser.add_argument('--out-version')
    parser.add_argument('--out-branch')

    args = parser.parse_args()

    with open(args.head_file) as fp:
        for line in map(str.strip, fp):
            continue

    heading, _ = line.split('\t')
    _, unix_time, _ = heading.rsplit(' ', 2)

    version = time.strftime('%y.%m.%d', time.gmtime(int(unix_time)))

    with open(args.out_version, 'w') as fp:
        fp.write(version)

    with open(args.out_branch, 'w') as fp:
        fp.write(args.branch)


if __name__ == '__main__':
    main()
