#!/usr/bin/bash

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -e
trap 'echo "$BASH_SOURCE:$LINENO: $BASH_COMMAND" failed, sorry.' ERR

. /run/host/nsbox/scripts/nsbox-apply-env.sh

latest_image=/run/host/nsbox/image
ansible_stamp=/var/lib/nsbox-container-state/ansible.stamp

# If the playbook has been modified since it was last run (or was never run), re-run it.
if [[ ! -f $ansible_stamp ]] || \
    # This works by checking if any image file is newer than our stamp file. If so,
    # find will run "false", which results in a non-zero return code.
    ! find /run/host/nsbox/image -newermm $ansible_stamp -exec false {} +; then
  # XXX: duplicated from nsbox-bender.py.
  branch="$(cat /run/host/nsbox/release/BRANCH)"
  version="$(cat /run/host/nsbox/release/VERSION)"

  if [[ "$branch" == "edge" ]]; then
    product_name="nsbox-edge"
  else
    product_name="nsbox"
  fi

  extra_vars=()
  extra_vars+=(ansible_python_interpreter=/usr/bin/python3)
  extra_vars+=(nsbox_branch=$branch)
  extra_vars+=(nsbox_version=$version)
  extra_vars+=(nsbox_product_name=$product_name)

  ANSIBLE_STDOUT_CALLBACK=default ansible-playbook --connection=local --inventory=localhost, \
    --extra-vars "${extra_vars[*]}" --skip-tags bend /run/host/nsbox/image/playbook.yaml

  mkdir -p $(dirname $ansible_stamp)
  touch $ansible_stamp
fi

exec sudo --user="$NSBOX_USER" /run/host/nsbox/scripts/nsbox-enter-run.sh "$@"
