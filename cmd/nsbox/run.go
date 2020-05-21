/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"os"

	"github.com/google/subcommands"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/daemon"
	"github.com/refi64/nsbox/internal/inventory"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/session"
)

type runCommand struct {
	container string
	restart   bool
	command   []string
}

func newRunCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &runCommand{})
}

func (*runCommand) Name() string {
	return "run"
}

func (*runCommand) Synopsis() string {
	return "run a container"
}

func (*runCommand) Usage() string {
	return `run [<container>] [<command...>]:
	Run a command within container. If a command is not given, the shell will be run. If a
	container is not given or is -, the default container will be run.
`
}

func (cmd *runCommand) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&cmd.restart, "restart", false, "Restart the container if it's already running")
}

func (cmd *runCommand) ParsePositional(fs *flag.FlagSet) error {
	if len(fs.Args()) >= 1 {
		cmd.container = fs.Args()[0]
		cmd.command = fs.Args()[1:]
	}

	return nil
}

func (cmd *runCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	var ct *container.Container
	var err error

	usrdata := app.(*nsboxApp).usrdata

	if cmd.container == "" || cmd.container == "-" {
		ct, err = inventory.DefaultContainer(usrdata)
		if ct == nil {
			err = errors.New("no default container is set")
		}
	} else {
		ct, err = container.Open(usrdata, cmd.container)
	}

	if err != nil {
		return args.HandleError(err)
	}

	if err := daemon.RunContainerViaTransientUnit(ct, cmd.restart, usrdata); err != nil {
		return args.HandleError(err)
	}

	log.Debug("Container presumed to be ready, entering...")

	exitCode, err := session.EnterContainer(ct, cmd.command, usrdata, app.(*nsboxApp).workdir)
	if err != nil {
		return args.HandleError(err)
	}

	os.Exit(exitCode)
	return subcommands.ExitSuccess // ?
}
