/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"

	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/inventory"
)

type setDefaultCommand struct {
	newDefault string
}

func newSetDefaultCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &setDefaultCommand{})
}

func (*setDefaultCommand) Name() string {
	return "set-default"
}

func (*setDefaultCommand) Synopsis() string {
	return "set the default container"
}

func (*setDefaultCommand) Usage() string {
	return `set-default [<default>]:
	Set the defualt container to the value of default. If - or the empty string is given as the
	new default, the default container will be unset.
`
}

func (*setDefaultCommand) SetFlags(fs *flag.FlagSet) {}

func (cmd *setDefaultCommand) ParsePositional(fs *flag.FlagSet) error {
	return args.ExpectArgs(fs, &cmd.newDefault)
}

func (cmd *setDefaultCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	if cmd.newDefault == "-" {
		cmd.newDefault = ""
	}

	if err := inventory.SetDefaultContainer(app.(*nsboxApp).usrdata, cmd.newDefault); err != nil {
		return args.HandleError(err)
	}

	return subcommands.ExitSuccess
}
