# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Generates host_paths.go, which is used to know how to access the daemon and scripts.

import argparse


TEMPLATE = '''
// This file was auto-generated. DO NOT EDIT.
// Use 'gn args' to change the directories there instead.

package paths

const State = "{state}"
const Libexec = "{libexec}"
const Share = "{share}"
'''.strip()


def main():
    parser = argparse.ArgumentParser()

    parser.add_argument('--state')
    parser.add_argument('--libexec')
    parser.add_argument('--share')
    parser.add_argument('--output')

    args = parser.parse_args()

    with open(args.output, 'w') as fp:
        fp.write(TEMPLATE.format_map(vars(args)))


if __name__ == '__main__':
    main()
