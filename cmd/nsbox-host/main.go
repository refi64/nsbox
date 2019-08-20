/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
	"github.com/refi64/nsbox/internal/ptyservice"
	"github.com/refi64/nsbox/internal/session"
	devnsbox "github.com/refi64/nsbox/internal/varlink"
	"github.com/varlink/go/varlink"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
)

var (
	app = kingpin.New("nsbox-host", "Tool for communicating with the nsbox host daemon")

	serviceCommand = app.Command("service", "").Hidden()
	serviceContainerName = serviceCommand.Arg("container", "").String()

	enterCommand = app.Command("enter", "").Hidden()
	enterStdin = enterCommand.Flag("stdin", "").String()
	enterStdout = enterCommand.Flag("stdout", "").String()
	enterStderr = enterCommand.Flag("stderr", "").String()
	enterUid = enterCommand.Flag("uid", "").Int()
	enterCwd = enterCommand.Flag("cwd", "").String()
	enterExec = enterCommand.Arg("exec", "").Strings()
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

	case enterCommand.FullCommand():
		err = session.ConnectPtys(*enterStdin, *enterStdout, *enterStderr)
		if err == nil {
			err = session.SetupContainerSession(*enterUid, *enterCwd, *enterExec)
		}
	}

	if err != nil {
		log.Fatal(err)
	}
}
