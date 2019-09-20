#!/bin/bash

git_token="$1"

set -ex

gn_args=(fedora_package=true fedora_guest_tools=true)
if [[ "$GITHUB_REF" == "*/stable" ]]; then
  git_branch=stable
  gn_args+=(is_stable_build=true)
else
  git_branch=master
fi

gn gen out --args="${gn_args[*]}"
ninja -C out rpm/nsbox{{,-guest-tools}.spec,-sources.tar}

git clone https://github%40nsbox.dev@github.com/nsbox-bot/rpm-spec-files -b $git_branch
cp \
  out/rpm/*.spec \
  out/rpm/nsbox-sources.tar \
  guest-tools/fedora/nsbox_trigger.py \
  rpm-spec-files
cd rpm-spec-files
git config user.email 'github@nsbox.dev'
git config user.name 'nsbox-bot'
git add .
git commit -am "Automated push at $(date)"

cat >askpass.sh <<'EOF'
echo "$git_token"
EOF
chmod +x askpass.sh

export git_token
GIT_ASKPASS=./askpass.sh git push
