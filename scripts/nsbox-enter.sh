#!/usr/bin/bash

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -e
trap 'echo "$BASH_SOURCE:$LINENO: $BASH_COMMAND" failed, sorry.' ERR

. /run/host/nsbox/shared-env

command='/run/host/nsbox/scripts/nsbox-enter-setup.sh "$@"'
# Order here is relevant to enter.go.
[[ -z "$1" ]] || command="setsid -c $command <$1"
[[ -z "$2" ]] || command="$command >$2"
[[ -z "$3" ]] || command="$command 2>$3"

shift 3
eval "exec $command"
