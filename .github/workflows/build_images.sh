#!/bin/bash

set -ex

dnf install -y ansible-bender buildah findutils git ninja-build podman unzip
sed -i 's/"overlay"/"vfs"/;s/^mountopt/#mountopt/' /etc/containers/storage.conf
sed -i 's/# events_logger = "journald"/events_logger = "file"/g;s/"systemd"/"cgroupfs"/' /usr/share/containers/libpod.conf

export _BUILDAH_STARTED_IN_USERNS="" BUILDAH_ISOLATION=chroot

curl -o gn.zip -L https://chrome-infra-packages.appspot.com/dl/gn/gn/linux-amd64/+/latest
unzip gn.zip gn
install -Dm 755 gn /usr/local/bin/gn
rm -f gn.zip gn

gn gen out
ninja -C out nsbox-edge-bender :install_share_release

IMAGES_TO_BUILD=(
  arch
  debian:buster
  fedora:31
)

for image in "${IMAGES_TO_BUILD[@]}"; do
  out/install/bin/nsbox-edge-bender images/"$image"
done

echo "$GCR_JSON_KEY" | podman login gcr.io -u _json_key --password-stdin
podman images --format '{{.Repository}}:{{.Tag}}' | grep '^gcr.io/nsbox-data' | grep -Ev -- '-(bud|failed)$' \
  | xargs -t -d$'\n' -n1 podman push
