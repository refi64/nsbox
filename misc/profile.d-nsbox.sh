# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

# Updates XDG_DATA_DIRS with nsbox container exports.

for dir in @STATE_DIR/nsbox/inventory/*; do
  [ -d "$dir/exports/share" ] || continue

  export XDG_DATA_DIRS="$XDG_DATA_DIRS:$dir/exports/share/"
done
