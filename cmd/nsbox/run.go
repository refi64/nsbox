/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"github.com/google/subcommands"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/daemon"
	"github.com/refi64/nsbox/internal/inventory"
	"github.com/refi64/nsbox/internal/session"
	"os"
)

type runCommand struct {
	container string
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
	return `run [-c container] <command...>:
	Run a command within container. If a command is not given, the shell will be run. If a
	container is not given, the default container set via set-default is run.
`
}

func (cmd *runCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&cmd.container, "c", "", "The container to run")
}

func (cmd *runCommand) ParsePositional(fs *flag.FlagSet) error {
	cmd.command = fs.Args()
	return nil
}

func (cmd *runCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	var ct *container.Container
	var err error

	usrdata := app.(*nsboxApp).usrdata

	if cmd.container == "" {
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

	if err := daemon.RunContainerViaTransientUnit(ct, usrdata); err != nil {
		return args.HandleError(err)
	}

	exitCode, err := session.EnterContainer(ct, cmd.command, usrdata, app.(*nsboxApp).workdir)
	if err != nil {
		return args.HandleError(err)
	}

	os.Exit(exitCode)
	return subcommands.ExitSuccess // ?
}
