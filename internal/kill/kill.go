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
	"strings"
)

// Package unix doesn't provide SIGRTMIN.

// #include <signal.h>
import "C"

type Signal unix.Signal

var (
	SigDefault  = Signal(0)
	// systemd sends SIGRTMIN + 4 to signify poweroff and SIGINT for reboot.
	SigPoweroff = Signal(C.SIGRTMIN + 4)
	SigReboot   = Signal(unix.SIGINT)
	SigTerm     = Signal(unix.SIGTERM)
	SigKill			= Signal(unix.SIGKILL)

	sigToString = map[Signal]string{
		SigDefault:  "default",
		SigPoweroff: "poweroff",
		SigReboot:   "reboot",
		SigTerm: 		 "sigterm",
		SigKill:     "sigkill",
	}

	stringToSig = map[string]Signal{
		"default" : SigDefault,
		"poweroff": SigPoweroff,
		"reboot"  : SigReboot,
		"term"    : SigTerm,
		"sigterm" : SigTerm,
		"kill"    : SigKill,
		"sigkill" : SigKill,
	}
)

func (sig Signal) String() string {
	return sigToString[sig]
}

func (sig *Signal) Set(value string) error {
	newSig, ok := stringToSig[strings.ToLower(value)]
	if !ok {
		return errors.New("does not exist")
	}

	*sig = newSig
	return nil
}

func KillContainer(usrdata *userdata.Userdata, name string, signal Signal, all bool) error {
	ct, err := container.Open(usrdata, name)
	if err != nil {
		return err
	}

	if ct.Config.Boot && all {
		return errors.New("-a/--all is not supported for booted containers")
	}

	if signal == SigDefault {
		if ct.Config.Boot {
			signal = SigPoweroff
		} else {
			signal = SigTerm
		}
	}

	if signal == SigPoweroff || signal == SigReboot {
		if !ct.Config.Boot {
			return errors.New("cannot send POWEROFF/REBOOT to a non-booted container")
		}
	} else if ct.Config.Boot {
		return errors.New("only POWEROFF/REBOOT may be sent to a booted container")
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

	if err := machined.KillMachine(name, who, unix.Signal(signal)); err != nil {
		return errors.Wrap(err, "failed to kill container")
	}

	if err := ct.LockUntilProcessDeath(container.WaitForLock); err != nil {
		return err
	}

	return nil
}
