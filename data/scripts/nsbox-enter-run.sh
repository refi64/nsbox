#!/bin/bash

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -e
trap 'echo "$BASH_SOURCE:$LINENO: $BASH_COMMAND" failed, sorry.' ERR

unset SUDO_COMMAND SUDO_USER SUDO_UID SUDO_GID
. /run/host/nsbox/scripts/nsbox-apply-env.sh

cwd="$1"
shift 1

if [[ -d "$cwd" ]]; then
  if [[ "$cwd" != */ ]]; then
    cwd="$cwd/"
  fi

  if [[ "$NSBOX_HOME_LINK_TARGET_ADJUST_CWD" == "1" \
        && "$cwd" == "/$NSBOX_HOME_LINK_TARGET"/* ]]; then
    cwd="$NSBOX_HOME_LINK_NAME/${cwd#/$NSBOX_HOME_LINK_TARGET/}"
  fi

  cd "$cwd"
fi

if [[ -n "$NSBOX_BOOTED" ]]; then
  # Booted systems have their own XDG_RUNTIME_DIR, we need to symlink relevant stuff inside.

  if [[ -n "$WAYLAND_DISPLAY" && ! -e "$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY" ]]; then
    ln -sf "/run/host/nsbox/usr-run/$WAYLAND_DISPLAY" "$XDG_RUNTIME_DIR"
  fi

  if [[ ! -e "$XDG_RUNTIME_DIR/pulse" ]]; then
    ln -sf "/run/host/nsbox/usr-run/pulse" "$XDG_RUNTIME_DIR"
  fi
fi

exec "$@"
