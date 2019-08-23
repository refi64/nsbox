#!/usr/bin/bash

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -e
trap 'echo "$BASH_SOURCE:$LINENO: $BASH_COMMAND" failed, sorry.' ERR

. /run/host/nsbox/scripts/nsbox-apply-env.sh

needed_packages=()

hash sudo &>/dev/null || needed_packages+=(sudo)
test -f /etc/profile.d/vte.sh || needed_packages+=(vte-profile)
test -f /usr/share/man/man3/errno.3.gz || needed_packages+=(man-pages)

if [[ ! -s /usr/bin/nsbox-host ]]; then
  echo "nsbox-enter: Installing: dnf-plugins-core"
  dnf install -y dnf-plugins-core

  dnf copr enable -y refi64/nsbox-guest-tools
  needed_packages+=(nsbox-guest-tools)
fi

if (( ${#needed_packages[@]} )); then
  echo "nsbox-enter: Installing: ${needed_packages[@]}"
  dnf install -y "${needed_packages[@]}"
fi

exec sudo --user="$NSBOX_USER" /run/host/nsbox/scripts/nsbox-enter-run.sh "$@"
