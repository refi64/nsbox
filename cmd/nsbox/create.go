/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/create"
)

type createCommand struct {
	name string
	version int
	boot bool
}

func newCreateCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &createCommand{})
}

func (*createCommand) Name() string {
	return "create"
}

func (*createCommand) Synopsis() string {
	return "create a new container"
}

func (*createCommand) Usage() string {
	return `create [-boot] [-version version] <container>:
	Creates a new container with the given name. You can provide an initial container config
	to it by passing various arguments.
`
}

func (cmd *createCommand) SetFlags(fs *flag.FlagSet) {
	fs.IntVar(&cmd.version, "version", 0, "The Fedora version to use (default is the host version)")
	fs.BoolVar(&cmd.boot, "boot", false, "Make the container a booted container")
}

func (cmd *createCommand) ParsePositional(fs *flag.FlagSet) error {
	return args.ExpectArgs(fs, &cmd.name)
}

func (cmd *createCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	var version string

	if cmd.version == 0 {
		version = ""
	} else {
		version = string(cmd.version)
	}

	config := container.Config{
		Boot: cmd.boot,
	}

	err := create.CreateContainer(app.(*nsboxApp).usrdata, cmd.name, version, config)
	return args.HandleError(err)
}
