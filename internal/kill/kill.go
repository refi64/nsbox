/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package kill

import (
	"os"
	"strings"

	"github.com/coreos/go-systemd/v22/machine1"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/userdata"
	"golang.org/x/sys/unix"
)

// Package unix doesn't provide SIGRTMIN.

// #include <signal.h>
import "C"

type Signal unix.Signal

var (
	// systemd sends SIGRTMIN + 4 to signify poweroff and SIGINT for reboot.
	SigPoweroff = Signal(C.SIGRTMIN + 4)
	SigKill     = Signal(unix.SIGKILL)

	sigToString = map[Signal]string{
		SigPoweroff: "poweroff",
		SigKill:     "sigkill",
	}

	stringToSig = map[string]Signal{
		"poweroff": SigPoweroff,
		"kill":     SigKill,
		"sigkill":  SigKill,
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

func KillContainer(usrdata *userdata.Userdata, ct *container.Container, signal Signal, all bool) error {
	if ct.Config.Boot && all {
		return errors.New("-a/--all is not supported for booted containers")
	}

	machined, err := machine1.New()
	if err != nil {
		return err
	}

	log.Debugf("sending signal %d to %s", int(signal))

	// machined's SELinux policies don't allow us to ask it to kill the leader process of a
	// container that machined didn't start. However, when "all" is given, machined just
	// forwards the kill request to systemd, which can kill any cgroup associated with a
	// systemd service. That means for "all", we can just forward the request to machined,
	// but otherwise, we need to send the kill signal ourselves.

	if all {
		if err := machined.KillMachine(ct.Name, "all", unix.Signal(signal)); err != nil {
			return errors.Wrap(err, "failed to ask machined to kill container")
		}
	} else {
		props, err := machined.DescribeMachine(ct.Name)
		if err != nil {
			return errors.Wrap(err, "failed to describe machine")
		}

		leader, err := os.FindProcess(int(props["Leader"].(uint32)))
		if err != nil {
			return errors.Wrap(err, "failed to find leader process")
		}

		log.Debug("leader process is", leader.Pid)

		if err := leader.Signal(unix.Signal(signal)); err != nil {
			return errors.Wrap(err, "failed to signal leader process")
		}
	}

	lock, err := ct.Lock(container.WaitForLock)
	if err != nil {
		return err
	}

	lock.Release()
	return nil
}
