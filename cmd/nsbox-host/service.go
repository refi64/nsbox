/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"github.com/coreos/go-systemd/daemon"
	"github.com/google/subcommands"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/ptyservice"
	devnsbox "github.com/refi64/nsbox/internal/varlink"
	"os"
)

func startPtyServiceAndNotifyHost(name string) error {
	if err := ptyservice.StartPtyService(name); err != nil {
		return errors.Wrap(err, "failed to start pty service")
  }

	conn, err := varlinkConnect()
	if err != nil {
		return err
  }

	defer conn.Close()

	if err := devnsbox.NotifyStart().Call(conn); err != nil {
		return errors.Wrap(err, "failed to notify of start")
  }

	if os.Getenv("NOTIFY_SOCKET") != "" {
		if _, err := daemon.SdNotify(true, daemon.SdNotifyReady); err != nil {
			return err
	 }
  }

	select {}
}

type serviceCommand struct {}

func newServiceCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &serviceCommand{})
}

func (*serviceCommand) Name() string {
	return "service";
}

func (*serviceCommand) Synopsis() string {
	return "INTERNAL COMMAND - starts the PTY service";
}

func (*serviceCommand) Usage() string {
	return "INTERNAL COMMAND - why the heck do you care about the usage, just DON'T USE IT\n";
}

func (*serviceCommand) SetFlags(fs *flag.FlagSet) {
}

func (*serviceCommand) Execute(fs *flag.FlagSet) subcommands.ExitStatus {
	var container string

	if !args.ExpectArgs(fs, &container) {
		return subcommands.ExitUsageError
	}

	return args.HandleError(startPtyServiceAndNotifyHost(container))
}
