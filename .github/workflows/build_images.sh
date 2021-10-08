#!/bin/bash

set -ex

dnf install -y ansible-bender buildah findutils git ninja-build podman unzip
sed -i 's/"overlay"/"vfs"/;s/^mountopt/#mountopt/' /etc/containers/storage.conf
sed -i 's/# \(cgroup_manager = \).*$/\1"cgroupfs"/' /usr/share/containers/containers.conf

export _BUILDAH_STARTED_IN_USERNS="" BUILDAH_ISOLATION=chroot

curl -o gn.zip -L https://chrome-infra-packages.appspot.com/dl/gn/gn/linux-amd64/+/latest
unzip gn.zip gn
install -Dm 755 gn /usr/local/bin/gn
rm -f gn.zip gn

# XXX: partly copied from .github/actions/rpm_spec_files/entrypoint.sh
gn_args=()
if [[ "$GITHUB_REF" == *"/stable" ]]; then
  gn_args+=(is_stable_build=true)
  bender=nsbox-bender
elif [[ "$GITHUB_REF" == *"/staging" ]]; then
  gn_args+=(is_stable_build=true)
  staging=1
  bender=nsbox-bender
else
  bender=nsbox-edge-bender
fi

gn gen out --args="${gn_args[*]}"
ninja -C out "$bender" :install_share_release

IMAGES_TO_BUILD=(
  arch
  debian:buster
  debian:bullseye
  fedora:33
  fedora:34
  fedora:35
)

for image in "${IMAGES_TO_BUILD[@]}"; do
  out/install/bin/"$bender" images/"$image"
done

list_images() {
  podman images --format '{{.Repository}}:{{.Tag}}' | grep '^gcr.io/nsbox-data' | grep -Ev -- '-(bud|failed)$'
}

push() {
  (set -x; podman push "$@")
}

echo "$GCR_JSON_KEY" | podman login gcr.io -u _json_key --password-stdin

for image in $(list_images); do
  if [[ -n "$staging" ]]; then
    push "$image" "$(echo "$image" | sed 's/\(:.*\)stable/\1staging/')"
  else
    push "$image"
  fi
done
