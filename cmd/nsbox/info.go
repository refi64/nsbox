/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"

	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/container"
)

type infoCommand struct {
	name string
}

func newInfoCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &infoCommand{})
}

func (*infoCommand) Name() string {
	return "info"
}

func (*infoCommand) Synopsis() string {
	return "show container info"
}

func (*infoCommand) Usage() string {
	return `info <container>
	Show information about the given container, including its running state and configuration.
`
}

func (*infoCommand) SetFlags(fs *flag.FlagSet) {}

func (cmd *infoCommand) ParsePositional(fs *flag.FlagSet) error {
	return args.ExpectArgs(fs, &cmd.name)
}

func (cmd *infoCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	err := container.OpenAndShowInfo(app.(*nsboxApp).usrdata, cmd.name)
	return args.HandleError(err)
}
