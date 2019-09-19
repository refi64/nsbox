/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/varlink/go/varlink"
)

func varlinkConnect() (*varlink.Connection, error) {
	conn, err := varlink.NewConnection("unix:///run/host/nsbox/" + paths.HostServiceSocketName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to host socket")
	}

	return conn, nil
}
