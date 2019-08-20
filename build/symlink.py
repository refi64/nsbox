# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Just symlinks a file. This script's boring-ness is only matched by bin_proxy.py.

import os
import sys

_, source, dest, stamp = sys.argv

if os.path.exists(dest):
    os.unlink(dest)

os.makedirs(os.path.dirname(dest), exist_ok=True)
os.symlink(source, dest)

with open(stamp, 'w') as fp:
    pass
