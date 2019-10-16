#!/usr/bin/bash

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -e
trap 'echo "$BASH_SOURCE:$LINENO: $BASH_COMMAND" failed, sorry.' ERR

. /run/host/nsbox/scripts/nsbox-apply-env.sh

stamp_root=/var/lib/nsbox-container-state/ansible
mkdir -p $stamp_root

get_images_to_update() {
  while [[ $# -gt 0 ]]; do
    local image=$1
    local stamp=$stamp_root/$image.stamp
    # find /run/host/nsbox/images/$image -newermm $stamp -exec false {} +

    # This messy if just checks if we need to run this playbook and, if so, then re-run all
    # the ones that follow as well.
    if [[ ! -f $stamp ]] || \
        # This works by checking if any image file is newer than our stamp file. If so,
        # find will run "false", which results in a non-zero return code. Therefore, we'll
        ! find /run/host/nsbox/images/$image -newermm $stamp -exec false {} +; then
      echo "$@"
      return
    fi

    shift
  done
}

to_update="$(get_images_to_update $NSBOX_IMAGE_CHAIN)"
if [[ -n "$to_update" ]]; then
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

  for image in $to_update; do
    ANSIBLE_STDOUT_CALLBACK=default ansible-playbook \
      --connection=local --inventory=localhost, --extra-vars "${extra_vars[*]}" --skip-tags bend \
      /run/host/nsbox/images/$image/playbook.yaml
    touch $stamp_root/$image.stamp
  done
fi

exec runuser -s /bin/bash - "$NSBOX_USER" /run/host/nsbox/scripts/nsbox-enter-run.sh "$@"
