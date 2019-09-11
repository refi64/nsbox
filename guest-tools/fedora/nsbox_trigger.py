# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Notifies the host to refresh the desktop files.

from dnfpluginscore import logger
import dnf
import dnf.cli

import subprocess

class NsboxTrigger(dnf.Plugin):
    name = 'nsbox-trigger'

    def transaction(self):
        logger.debug('Notifying nsbox host of updates...')
        subprocess.run(['nsbox-host', 'reload-exports'])
