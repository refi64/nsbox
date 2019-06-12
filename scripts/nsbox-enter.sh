#!/usr/bin/bash

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -e

user="$NSBOX_USER"
uid="$NSBOX_UID"
shell="$NSBOX_SHELL"
cwd="$NSBOX_CWD"

read -r -d '' setup_user_env <<'EOF' ||:
. /run/host/nsbox/env
unset SUDO_COMMAND SUDO_USER SUDO_UID SUDO_GID

[[ -n "$1" ]] && cd "$1" ||:
shift 1

exec "$@"
EOF

needed_packages=()

hash sudo &>/dev/null || needed_packages+=(sudo)

trap 'echo "$BASH_SOURCE:$LINENO: $BASH_COMMAND" failed, sorry.' ERR

if (( ${#needed_packages[@]} )); then
  echo "nsbox-enter: Installing ${needed_packages[@]}..."
  dnf install -y "${needed_packages[@]}"
fi

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

if [[ -d "$cwd" ]]; then
  if [[ "$cwd" != */ ]]; then
    cwd="$cwd/"
  fi

  if [[ "$cwd" == "$NSBOX_HOME_LINK_TARGET"/* ]]; then
    cwd="$NSBOX_HOME_LINK_NAME/${cwd#$NSBOX_HOME_LINK_TARGET/}"
  fi
else
  cwd=""
fi

ln -s /var/log/journal/$NSBOX_HOST_MACHINE /run/host/journal

exec sudo --user="$user" \
  NSBOX_HOST_MACHINE="$NSBOX_HOST_MACHINE" \
  NSBOX_CONTAINER="$NSBOX_CONTAINER" \
  bash -c "$setup_user_env" nsbox-helper "$cwd" "$@"
