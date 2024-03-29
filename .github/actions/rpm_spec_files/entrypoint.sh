#!/bin/bash

git_token="$1"

set -ex

gn_args=(fedora_package=true)
if [[ "$GITHUB_REF" == *"/stable" ]]; then
  git_branch=stable
  gn_args+=(is_stable_build=true)
elif [[ "$GITHUB_REF" == *"/staging" ]]; then
  git_branch=staging
  gn_args+=(is_stable_build=true)
else
  git_branch=master
fi

go mod vendor -v

gn gen out --args="${gn_args[*]}"
ninja -C out rpm/nsbox{.spec,-sources.tar}

git clone https://github%40nsbox.dev@github.com/nsbox-bot/rpm-spec-files
if git rev-parse --verify origin/$git_branch 2>/dev/null; then
  git checkout $git_branch
else
  git checkout -b $git_branch
fi

cp \
  out/rpm/nsbox.spec \
  out/rpm/nsbox-sources.tar \
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
