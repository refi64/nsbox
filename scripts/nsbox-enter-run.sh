#!/usr/bin/bash

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -e
trap 'echo "$BASH_SOURCE:$LINENO: $BASH_COMMAND" failed, sorry.' ERR

unset SUDO_COMMAND SUDO_USER SUDO_UID SUDO_GID
. /run/host/nsbox/shared-env

cwd="$1"
shift 1

if [[ -d "$cwd" ]]; then
  if [[ "$cwd" != */ ]]; then
    cwd="$cwd/"
  fi

  if [[ "$cwd" == "$NSBOX_HOME_LINK_TARGET"/* ]]; then
    cwd="$NSBOX_HOME_LINK_NAME/${cwd#$NSBOX_HOME_LINK_TARGET/}"
  fi

  cd "$cwd"
fi

exec "$@"
