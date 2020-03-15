# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Provides some basic helpers for parsing the JSON output of 'go list -mod=...' commands.

import json
import subprocess

def go_list(go, package, *, cwd=None, vendor=False, tags=set()):
    mod_arg = 'vendor' if vendor else 'readonly'

    process = subprocess.run([go, 'list', f'-mod={mod_arg}', '-json', '-deps',
                              f'-tags={" ".join(tags)}', package],
                             stdout=subprocess.PIPE, universal_newlines=True, check=True,
                             cwd=cwd)

    # Change formatting to be in list form. (Go list prints JSON objects back-to-back.)
    dep_json = '[' + process.stdout.replace('\n}', '\n},').rstrip(',\n') + ']'
    return json.loads(dep_json)
