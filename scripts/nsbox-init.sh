#!/usr/bin/bash

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -e
trap 'echo "$BASH_SOURCE:$LINENO: $BASH_COMMAND" failed, sorry.' ERR

. /run/host/nsbox/shared-env

user="$NSBOX_USER"
uid="$NSBOX_UID"
shell="$NSBOX_SHELL"

mkdir -p /etc/nsbox-state
if [[ ! -f /etc/nsbox-state/remove-nodocs ]]; then
  sed -i '/tsflags=nodocs/d' /etc/dnf/dnf.conf
  touch /etc/nsbox-state/remove-nodocs
fi

needed_packages=()

hash sudo &>/dev/null || needed_packages+=(sudo)
test -f /etc/profile.d/vte.sh || needed_packages+=(vte-profile)
test -f /usr/share/man/man3/errno.3.gz || needed_packages+=(man-pages)

if (( ${#needed_packages[@]} )); then
  echo "nsbox-enter: Installing: ${needed_packages[@]}"
  dnf install -y "${needed_packages[@]}"
fi

ps -A

groups=$(cat /run/host/nsbox/supplementary-groups /etc/group | cut -d: -f3 | sort | uniq -d \
         | head -c -1 | tr '\n' ',')
grep -q "$user" /etc/passwd && userdel "$user" ||:
rm -f /var/mail/"$user"
useradd -M -G "$groups" -u "$uid" -s "$shell" "$user"

if [[ -d /run/host/nsbox/mail ]]; then
  rm -f /var/mail/"$user"
  ln -s /run/host/nsbox/mail /var/mail/"$user"
fi

if grep -q "^$user" /etc/shadow; then
  grep -v "$user" /etc/shadow > /etc/shadow.x
  mv /etc/shadow{.x,}
fi

cp /etc/shadow{,.x}
grep "^$user" /run/host/etc/shadow >> /etc/shadow.x
mv /etc/shadow{.x,}

if [[ -n "$NSBOX_HOME_LINK_NAME" ]]; then
  [[ -e "$NSBOX_HOME_LINK_NAME" ]] && rm -d "$NSBOX_HOME_LINK_NAME" ||:
  ln -s "$NSBOX_HOME_LINK_TARGET" "$NSBOX_HOME_LINK_NAME"
fi

ln -s /var/log/journal/$NSBOX_HOST_MACHINE /run/host/journal

# XXX: there's got to be a better way to do...all of this
touch /run/host/nsbox/start-notify
exec sleep infinity
