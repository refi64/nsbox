#!/bin/bash

branch="$1"
git_token="$2"

gn_args=(fedora_package=true fedora_guest_tools=true)
if [[ "$branch" == "stable" ]]; then
  git_branch=stable
  gn_args+=(is_stable_build=true)
else
  git_branch=master
fi

gn gen out --args="${gn_args[@]}"
ninja -C out rpm/nsbox{,-guest-tools}.spec

git clone https://nsbox-bot:"$git_token"@github.com/nsbox-bot/rpm-spec-files -b $git_branch
cp out/rpm/*.spec rpm-spec-files
cd rpm-spec-files
git add .
git commit -am "Automated push at $(date)"
git push
