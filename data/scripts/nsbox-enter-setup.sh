#!/bin/bash

# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -e
trap 'echo "$BASH_SOURCE:$LINENO: $BASH_COMMAND" failed, sorry.' ERR

. /run/host/nsbox/scripts/nsbox-apply-env.sh

integrity_root=/var/lib/nsbox-container-state/ansible
mkdir -p $integrity_root

get_images_to_update() {
  while [[ $# -gt 0 ]]; do
    local image=$1
    local integrity_file=$integrity_root/$image.sha256

    # If the current image has no previous sha256s saved, or any of them have changed since last
    # time, then this image and all its dependents will have their playbooks re-run.
    if [[ ! -f "$integrity_file" ]] || ! sha256sum --status -c $integrity_file; then
      # Generate a new integrity file.
      find /run/host/nsbox/images/$image -type f | xargs sha256sum > $integrity_file.tmp
      # It will be renamed below if the ansible playbook runs successfully.

      echo "$@"
      return
    fi

    shift
  done
}

if [[ -z "$NSBOX_NO_REPLAY" ]]; then
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
      # XXX: ugly duplication, but this is getting nuked in favor of a Python variant later on anyway
      mv $integrity_root/$image.sha256{.tmp,}
    done
  fi
fi

exec runuser -s /bin/bash -- - "$NSBOX_USER" /run/host/nsbox/scripts/nsbox-enter-run.sh "$@"
