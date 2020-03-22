/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"context"
	"flag"
	"os"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/google/subcommands"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/ptyservice"
	devnsbox "github.com/refi64/nsbox/internal/varlink"
)

func startPtyServiceAndNotifyHost(name string) error {
	if err := ptyservice.StartPtyService(name); err != nil {
		return errors.Wrap(err, "failed to start pty service")
	}

	conn, err := varlinkConnect()
	if err != nil {
		return err
	}

	// NOTE: the connection is not closed because we will generally never die normally:
	// - If an error occurs, then it needs to be logged before the connection is closed, at
	// 	 which point nsboxd dies before we can log the message. On error, nsbox-host dies
	//   anyway.
	// - If no error occurs, this will run forever until killed.

	if err := devnsbox.NotifyStart().Call(context.Background(), conn); err != nil {
		return errors.Wrap(err, "failed to notify of start")
	}

	if os.Getenv("NOTIFY_SOCKET") != "" {
		if _, err := daemon.SdNotify(true, daemon.SdNotifyReady); err != nil {
			return err
		}
	}

	select {}
}

type serviceCommand struct {
	container string
}

func newServiceCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &serviceCommand{})
}

func (*serviceCommand) Name() string {
	return "service"
}

func (*serviceCommand) Synopsis() string {
	return "INTERNAL COMMAND - starts the PTY service"
}

func (*serviceCommand) Usage() string {
	return "INTERNAL COMMAND - why the heck do you care about the usage, just DON'T USE IT\n"
}

func (*serviceCommand) SetFlags(fs *flag.FlagSet) {
}

func (cmd *serviceCommand) ParsePositional(fs *flag.FlagSet) error {
	return args.ExpectArgs(fs, &cmd.container)
}

func (cmd *serviceCommand) Execute(_ args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	return args.HandleError(startPtyServiceAndNotifyHost(cmd.container))
}
