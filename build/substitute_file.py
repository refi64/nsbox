# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Runs template substitutions over a file.

import argparse
import string


class CustomTemplate(string.Template):
    delimiter = '@'


def main():
    parser = argparse.ArgumentParser()

    parser.add_argument('--source')
    parser.add_argument('--dest')
    parser.add_argument('--state')
    parser.add_argument('--libexec')
    parser.add_argument('--share')
    parser.add_argument('vars', nargs=argparse.REMAINDER)

    args = parser.parse_args()

    substitutions = dict(var.split('=', 1) for var in args.vars)

    with open(args.source) as source, open(args.dest, 'w') as dest:
        for line in source:
            if line.strip():
                line = CustomTemplate(line).substitute(substitutions)

            dest.write(line)


if __name__ == '__main__':
    main()
