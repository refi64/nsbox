/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package kill

import (
	"github.com/coreos/go-systemd/machine1"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func KillContainer(name, sigstr string) error {
	var signal unix.Signal

	if sigstr == "SIGTERM" {
		signal = unix.SIGTERM
	} else if sigstr == "SIGKILL" {
		signal = unix.SIGKILL
	} else {
		panic("invalid signal string: " + sigstr)
	}

	machined, err := machine1.New()
	if err != nil {
		return err
	}

	if err := machined.KillMachine(name, "all", signal); err != nil {
		return errors.Wrap(err, "failed to kill container")
	}

	return nil
}
