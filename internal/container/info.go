/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package container

import (
	"fmt"
	"github.com/coreos/go-systemd/dbus"
	"github.com/coreos/go-systemd/machine1"
	"github.com/dustin/go-humanize"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/userdata"
	"os"
	"text/tabwriter"
	"time"
)

func boolYesNo(value bool) string {
	if value {
		return "yes"
	} else {
		return "no"
	}
}

func (ct Container) ShowInfo() error {
	systemd, err := dbus.New()
	if err != nil {
		return err
	}

	machined, err := machine1.New()
	if err != nil {
		return err
	}

	unitMemory, err := systemd.GetServiceProperty(fmt.Sprintf("nsbox-%s.service", ct.Name), "MemoryCurrent")
	if err != nil {
		log.Debug("failed to get unit MemoryCurrent:", err)
	}

	machineProps, err := machined.DescribeMachine(ct.Name)
	if err != nil {
		log.Debug("failed to describe machine:", err)
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 2, 1, ' ', tabwriter.AlignRight)
	defer writer.Flush()

	fmt.Fprintln(writer, "Name:\t", ct.Name)
	fmt.Fprintln(writer, "Booted:\t", boolYesNo(ct.Config.Boot))

	if machineProps != nil {
		usec := machineProps["Timestamp"].(uint64)
		tm := time.Unix(int64(usec) / int64(time.Second / time.Microsecond), 0)
		fmt.Fprintf(writer, "Running:\t since %s (%s)\n", tm.Format(time.RFC1123), humanize.Time(tm))
	} else {
		fmt.Fprintln(writer, "Running:\t no")
	}

	if unitMemory != nil {
		memory := unitMemory.Value.Value().(uint64)
		fmt.Fprintln(writer, "Memory:\t", humanize.Bytes(memory))
	}

	return nil
}

func OpenAndShowInfo(usrdata *userdata.Userdata, name string) error {
	ct, err := Open(usrdata, name)
	if err != nil {
		return err
	}

	return ct.ShowInfo()
}
