/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"fmt"
	"github.com/google/subcommands"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/inventory"
	"strings"
)

type deleteCommand struct {
	name string
	yes  bool
}

func newDeleteCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &deleteCommand{})
}

func (*deleteCommand) Name() string {
	return "delete"
}

func (*deleteCommand) Synopsis() string {
	return "delete a container"
}

func (*deleteCommand) Usage() string {
	return `delete [-y] <container>
	Permanently deletes the given container.
`
}

func (cmd *deleteCommand) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&cmd.yes, "y", false, "Don't ask to confirm whether or not to delete the container")
}

func (cmd *deleteCommand) ParsePositional(fs *flag.FlagSet) error {
	return args.ExpectArgs(fs, &cmd.name)
}

func (cmd *deleteCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	ct, err := container.Open(app.(*nsboxApp).usrdata, cmd.name)
	if err != nil {
		return args.HandleError(err)
	}

	def, err := inventory.DefaultContainer(app.(*nsboxApp).usrdata)
	if err != nil {
		return args.HandleError(err)
	}

	if def != nil && def.Name == ct.Name {
		return args.HandleError(errors.New("Cannot delete the default container."))
	}

	if !cmd.yes {
		fmt.Printf("Are you sure you want to PERMANENTLY delete %s? (y/n) ", cmd.name)

		var resp string
		fmt.Scanln(&resp)
		if strings.ToLower(resp) != "y" {
			return subcommands.ExitSuccess
		}
	}

	return args.HandleError(ct.LockAndDelete(container.NoWaitForLock))
}
