# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Generates a "Python binary" by compiling the Python to bytecode and adding a wrapper
# script.

import argparse
import os
import py_compile
import shlex
import shutil


SCRIPT_WRAPPER = '''
#!/bin/bash

python3 "$(dirname "$0")/"{0} "$@"
'''.lstrip()


def main():
    parser = argparse.ArgumentParser()

    parser.add_argument('--script')
    parser.add_argument('--out-wrapper')
    parser.add_argument('--out-py')
    parser.add_argument('--out-pyc')

    args = parser.parse_args()

    shutil.copy(args.script, args.out_py)
    py_compile.compile(args.out_py, args.out_pyc)

    relative_compiled_script = os.path.relpath(args.out_pyc,
                                               os.path.dirname(args.out_wrapper))

    with open(args.out_wrapper, 'w') as fp:
        fp.write(SCRIPT_WRAPPER.format(shlex.quote(relative_compiled_script)))

    os.chmod(args.out_wrapper, 0o755)


if __name__ == '__main__':
    main()
