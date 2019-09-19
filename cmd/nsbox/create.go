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
	image string
	name  string
	tar   string
	boot  bool
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
	return `create [-boot] [-tar <tar>] <image> <container>:
	Creates a new container with the given name from the given image. You can provide an initial
	container config to it by passing various arguments.
`
}

func (cmd *createCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&cmd.tar, "tar", "", "Override the image contents with this tar file")
	fs.BoolVar(&cmd.boot, "boot", false, "Make the container a booted container")
}

func (cmd *createCommand) ParsePositional(fs *flag.FlagSet) error {
	return args.ExpectArgs(fs, &cmd.image, &cmd.name)
}

func (cmd *createCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	config := container.Config{
		Image: cmd.image,
		Boot:  cmd.boot,
	}

	err := create.CreateContainer(app.(*nsboxApp).usrdata, cmd.name, cmd.tar, config)
	return args.HandleError(err)
}
