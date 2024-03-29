/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// Run a container indirectly, by starting a transient systemd service that runs nsboxd.
package daemon

import (
	"fmt"
	"os"
	"time"

	systemd1 "github.com/coreos/go-systemd/v22/dbus"
	"github.com/coreos/go-systemd/v22/machine1"
	"github.com/coreos/go-systemd/v22/sdjournal"
	godbus "github.com/godbus/dbus/v5"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/kill"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/nsbus"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/selinux"
	"github.com/refi64/nsbox/internal/userdata"
)

type temporaryFileSystem struct{ Path, Options string }

func startNsboxd(systemd *systemd1.Conn, nsboxd string, ct *container.Container, usrdata *userdata.Userdata) error {
	serviceName := fmt.Sprintf("nsbox-%s.service", ct.MachineName(usrdata))

	journal, err := sdjournal.NewJournalReader(sdjournal.JournalReaderConfig{
		// XXX: use a 1-nanosecond duration to get it to filter starting now.
		// If it's 0, then NewJournalReader will think it's completely unset.
		Since: 1,
		Matches: []sdjournal.Match{
			{
				Field: sdjournal.SD_JOURNAL_FIELD_SYSTEMD_UNIT,
				Value: serviceName,
			},
		},
		Formatter: func(entry *sdjournal.JournalEntry) (string, error) {
			msg, ok := entry.Fields["MESSAGE"]
			if !ok {
				return "", errors.Errorf("Journal entry had no MESSAGE field")
			}

			return fmt.Sprintln(msg), nil
		},
	})

	if err != nil {
		return errors.Wrap(err, "opening journal reader")
	}

	// If a unit reset failed, it likely just never was running.
	_ = systemd.ResetFailedUnit(serviceName)

	xdgRuntimeDir, err := getXdgRuntimeDir(usrdata)
	if err != nil {
		return err
	}
	env := append([]string{"PKEXEC_UID=" + usrdata.User.Uid, "XDG_RUNTIME_DIR=" + xdgRuntimeDir})

	properties := []systemd1.Property{
		systemd1.PropType("notify"),
		systemd1.PropDescription(fmt.Sprintf("nsbox container %s for %s", ct.Name, usrdata.User.Username)),
		systemd1.PropExecStart(
			[]string{nsboxd, fmt.Sprint("-v=", log.Verbose()), ct.Name},
			false,
		),
		{
			// This is needed for safety with use of nsbus, see there for more info.
			Name:  "TemporaryFileSystem",
			Value: godbus.MakeVariant([]temporaryFileSystem{{Path: nsbus.PrivateBusTmpdir}}),
		},
		{
			Name:  "Environment",
			Value: godbus.MakeVariant(env),
		},
		{
			Name:  "NotifyAccess",
			Value: godbus.MakeVariant("all"),
		},
	}

	if ct.Config.VirtualNetwork {
		properties = append(properties, systemd1.PropRequires("systemd-networkd.service"))
	}

	journalUntil := make(chan time.Time)
	jobStatus := make(chan string)

	_, err = systemd.StartTransientUnit(serviceName, "replace", properties, jobStatus)
	if err != nil {
		return errors.Wrap(err, "starting transient unit")
	}

	go func() {
		if err := journal.Follow(journalUntil, os.Stdout); err != sdjournal.ErrExpired {
			log.Info("failed to follow journal: ", err)
		}
	}()

	jobResult := <-jobStatus
	// XXX: Make sure the log output gets fully flushed out.
	time.Sleep(time.Millisecond * 700)
	journalUntil <- time.Now()

	if jobResult != "done" {
		if selinux.Enforcing() {
			log.Alert("NOTE: If there is a permission denied error, try setting SELinux to permissive.")
			log.Alert("If that works, please file a bug report with nsbox.")
		}
		return fmt.Errorf("job failed (see 'systemctl status %s' for more info)", serviceName)
	}

	return nil
}

func RunContainerViaTransientUnit(ct *container.Container, restart bool, usrdata *userdata.Userdata) error {
	ct.ApplyEnvironFilter(usrdata)

	systemd, err := systemd1.NewSystemConnection()
	if err != nil {
		return err
	}

	machined, err := machine1.New()
	if err != nil {
		return err
	}

	if restart {
		if _, err := machined.GetMachine(ct.MachineName(usrdata)); err == nil {
			log.Debug("Killing previous container instance")

			var signal kill.Signal
			var all bool
			if ct.Config.Boot {
				signal = kill.SigPoweroff
				all = false
			} else {
				signal = kill.SigKill
				all = true
			}

			if err := kill.KillContainer(usrdata, ct, signal, all); err != nil {
				return errors.Wrap(err, "killing previous instance")
			}
		}
	}

	if _, err := machined.GetMachine(ct.MachineName(usrdata)); err != nil {
		log.Debug("GetMachine:", err)

		nsboxd, err := paths.GetPrivateExecutable("nsboxd")
		if err != nil {
			return errors.Wrap(err, "cannot locate nsboxd")
		}

		if err := startNsboxd(systemd, nsboxd, ct, usrdata); err != nil {
			return errors.Wrap(err, "cannot start nsboxd")
		}
	}

	return nil
}
