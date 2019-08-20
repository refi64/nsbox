/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/ptyservice"
	devnsbox "github.com/refi64/nsbox/internal/varlink"
	"github.com/varlink/go/varlink"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
)

var (
	app = kingpin.New("nsbox-host", "Tool for communicating with the nsbox host daemon")

	serviceCommand = app.Command("service", "").Hidden()
	serviceContainerName = serviceCommand.Arg("container", "").String()
)

func startPtyServiceAndNotifyHost(name string) error {
	if err := ptyservice.StartPtyService(name); err != nil {
		return errors.Wrap(err, "failed to start pty service")
	}

	conn, err := varlink.NewConnection("unix:///run/host/nsbox/" + paths.HostServiceSocketName)
	if err != nil {
		return errors.Wrap(err, "failed to connect to host socket")
	}

	if err := devnsbox.NotifyStart().Call(conn); err != nil {
		return errors.Wrap(err, "failed to notify of start")
	}

	select {}
}

func main() {
	app.HelpFlag.Short('h')

	var err error

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case serviceCommand.FullCommand():
		err = startPtyServiceAndNotifyHost(*serviceContainerName)
	}

	if err != nil {
		log.Fatal(err)
	}
}
