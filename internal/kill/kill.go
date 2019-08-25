/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package kill

import (
	"github.com/coreos/go-systemd/machine1"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/userdata"
	"golang.org/x/sys/unix"
)

// Package unix doesn't provide SIGRTMIN.

// #include <signal.h>
import "C"

// systemd sends SIGRTMIN + 4 to signify poweroff and SIGINT for reboot.

var (
	SIGRTMIN = unix.Signal(C.SIGRTMIN)
	POWEROFF = unix.Signal(int(SIGRTMIN) + 4)
	REBOOT   = unix.SIGINT
)

func KillContainer(usrdata *userdata.Userdata, name, sigstr string, all bool) error {
	ct, err := container.Open(usrdata, name)
	if err != nil {
		return err
	}

	if ct.Config.Boot && all {
		return errors.New("-a/--all is not supported for booted containers")
	}

	var signal unix.Signal

	if sigstr == "" {
		if ct.Config.Boot {
			sigstr = "POWEROFF"
		} else {
			sigstr = "SIGTERM"
		}
	}

	if sigstr == "POWEROFF" || sigstr == "REBOOT" {
		if !ct.Config.Boot {
			return errors.New("cannot send POWEROFF/REBOOT to a non-booted container")
		}

		if sigstr == "POWEROFF" {
			signal = POWEROFF
		} else {
			signal = REBOOT
		}
	} else {
		if ct.Config.Boot {
			return errors.New("only POWEROFF/REBOOT may be sent to a booted container")
		}

		if sigstr == "SIGTERM" {
			signal = unix.SIGTERM
		} else if sigstr == "SIGKILL" {
			signal = unix.SIGKILL
		} else {
			panic("invalid signal string: " + sigstr)
		}
	}

	machined, err := machine1.New()
	if err != nil {
		return err
	}

	var who string
	if all {
		who = "all"
	} else {
		who = "leader"
	}

	log.Debugf("sending signal %d to %s", int(signal), who)

	if err := machined.KillMachine(name, who, signal); err != nil {
		return errors.Wrap(err, "failed to kill container")
	}

	if err := ct.LockUntilProcessDeath(container.WaitForLock); err != nil {
		return err
	}

	return nil
}
