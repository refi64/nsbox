/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// We want the go interface generator sources vendored, but:
// - This can't manually be added to go.mod because the next tidy will drop it.
// - This can't be imported from our main modules because it's not actually a library.
// So I did the next best thing: create a stub .go file that imports it, that way it'll
// be scanned by 'go mod' and be added normally.

package force_varlink

import (
	_ "github.com/varlink/go/cmd/varlink-go-interface-generator"
)
