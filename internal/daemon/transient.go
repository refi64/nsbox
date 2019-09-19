/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// Run a container indirectly, by starting a transient systemd service that runs nsboxd.
package daemon

import (
	"fmt"
	systemd1 "github.com/coreos/go-systemd/dbus"
	"github.com/coreos/go-systemd/machine1"
	"github.com/coreos/go-systemd/sdjournal"
	godbus "github.com/godbus/dbus"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/userdata"
	"os"
	"time"
)

func startNsboxd(systemd *systemd1.Conn, nsboxd, name string, usrdata *userdata.Userdata) error {
	serviceName := fmt.Sprintf("nsbox-%s.service", name)

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

	// These errors aren't that significant.
	_ = systemd.ResetFailedUnit(serviceName)

	env := append([]string{"SUDO_UID=" + usrdata.User.Uid}, userdata.WhitelistedEnviron()...)

	properties := []systemd1.Property{
		systemd1.PropType("notify"),
		systemd1.PropDescription("nsbox " + name),
		systemd1.PropExecStart(
			[]string{nsboxd, fmt.Sprint("-v=", log.Verbose()), name},
			false,
		),
		systemd1.Property{
			Name:  "Environment",
			Value: godbus.MakeVariant(env),
		},
		systemd1.Property{
			Name:  "NotifyAccess",
			Value: godbus.MakeVariant("all"),
		},
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
		return fmt.Errorf("job failed (see 'systemctl status %s' for more info)", serviceName)
	}

	return nil
}

func RunContainerViaTransientUnit(ct *container.Container, usrdata *userdata.Userdata) error {
	ct.ApplyEnvironFilter(usrdata)

	systemd, err := systemd1.NewSystemConnection()
	if err != nil {
		return err
	}

	machined, err := machine1.New()
	if err != nil {
		return err
	}

	if _, err := machined.GetMachine(ct.Name); err != nil {
		nsboxd, err := paths.GetPathRelativeToInstallRoot(paths.Libexec, paths.ProductName, "nsboxd")
		if err != nil {
			return errors.Wrap(err, "cannot locate nsboxd")
		}

		if err := startNsboxd(systemd, nsboxd, ct.Name, usrdata); err != nil {
			return errors.Wrap(err, "cannot start nsboxd")
		}
	}

	return nil
}
