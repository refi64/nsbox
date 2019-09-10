/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/kill"
)

type killCommand struct {
	container string
	signal kill.Signal
	all bool
}

func newKillCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &killCommand{})
}

func (*killCommand) Name() string {
	return "kill"
}

func (*killCommand) Synopsis() string {
	return "kill a container"
}

func (*killCommand) Usage() string {
	return `kill [-signal signal] [-all] <container>:
	Kill the container using the given signal. If the signal is not given, then poweroff is the
	default for booted containers, and sigterm is the default for non-booted containers.
`
}

func (cmd *killCommand) SetFlags(fs *flag.FlagSet) {
	fs.Var(&cmd.signal, "signal", "The signal to use to kill the container")
	fs.BoolVar(&cmd.all, "all", false, "Send the signal to all processes, not just the leader")
}

func (cmd *killCommand) ParsePositional(fs *flag.FlagSet) error {
	return args.ExpectArgs(fs, &cmd.container)
}

func (cmd *killCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	err := kill.KillContainer(app.(*nsboxApp).usrdata, cmd.container, cmd.signal, cmd.all)
	return args.HandleError(err)
}
