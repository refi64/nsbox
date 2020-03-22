#!/bin/bash

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -e
trap 'echo "$BASH_SOURCE:$LINENO: $BASH_COMMAND" failed, sorry.; exit 1' ERR

. /run/host/nsbox/scripts/nsbox-apply-env.sh

user="$NSBOX_USER"
uid="$NSBOX_UID"
shell="$NSBOX_SHELL"

rm -f /var/mail/"$user"

if grep -q "^$user:" /etc/passwd; then
  usermod -G "$NSBOX_SUDO_GROUP" -u "$uid" -s "$shell" "$user"
else
  useradd -MU -G "$NSBOX_SUDO_GROUP" -u "$uid" -s "$shell" "$user"
fi

if [[ -d /run/host/nsbox/mail ]]; then
  rm -f /var/mail/"$user"
  ln -s /run/host/nsbox/mail /var/mail/"$user"
fi

update=1

# XXX: shadow file hacks suck, but the only real workarond is to define a custom
# NSS module that asks the host, which...is not very fun.
if [[ -f /run/host/nsbox/shadow-custom-pass ]]; then
  # https://stackoverflow.com/questions/407523/escape-a-string-for-a-sed-replace-pattern
  pass=$(sed -e 's/[\/&]/\\&/g' /run/host/nsbox/shadow-custom-pass)
  sed "s/^\($user\):[^:]*/\1:$pass/" /etc/shadow > /etc/shadow.x
  unset pass
elif [[ -f /run/host/nsbox/shadow-entry ]]; then
  grep -v "^$user" /etc/shadow > /etc/shadow.x
  cat /run/host/nsbox/shadow-entry >> /etc/shadow.x
else
  update=
fi

if [[ -n "$update" ]]; then
  rm -f /run/host/nsbox/shadow-{custom-pass,entry}
  mv /etc/shadow{.x,}
  chmod 000 /etc/shadow
fi

if [[ "$NSBOX_BOOTED" == "1" ]]; then
  hostnamectl set-hostname "$HOSTNAME"
else
  echo "$HOSTNAME" > /etc/hostname
fi

ln -sf {/run/host,}/etc/locale.conf

if [[ -n "$NSBOX_HOME_LINK_NAME" ]]; then
  [[ -e "$NSBOX_HOME_LINK_NAME" ]] && rm -d "$NSBOX_HOME_LINK_NAME" ||:
  ln -s "$NSBOX_HOME_LINK_TARGET_REL" "$NSBOX_HOME_LINK_NAME"
fi

ln -sf /var/log/journal/$NSBOX_HOST_MACHINE /run/host/journal

if [[ -n "$NSBOX_BOOTED" ]]; then
  rm -f "$XDG_RUNTIME_DIR"/wayland-*
fi

exec /run/host/nsbox/nsbox-host service "$NSBOX_CONTAINER"
