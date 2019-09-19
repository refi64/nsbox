# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Parses the date of the latest git commit to generate the version.

import argparse
import json
import time


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--head-file')

    args = parser.parse_args()

    with open(args.head_file) as fp:
        for line in map(str.strip, fp):
            continue

    heading, _ = line.split('\t')
    _, unix_time, _ = heading.rsplit(' ', 2)

    version = time.strftime('%y.%m.%d', time.gmtime(int(unix_time)))
    print(version, end='')


if __name__ == '__main__':
    main()
