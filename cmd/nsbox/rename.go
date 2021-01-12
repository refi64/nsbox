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
	"github.com/refi64/nsbox/internal/inventory"
	"github.com/refi64/nsbox/internal/userdata"
)

type renameCommand struct {
	current string
	new     string
}

func newRenameCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &renameCommand{})
}

func (*renameCommand) Name() string {
	return "rename"
}

func (*renameCommand) Synopsis() string {
	return "rename a container"
}

func (*renameCommand) Usage() string {
	return `rename <container> <new>
	 Rename the given container to a new name.
`
}

func (*renameCommand) SetFlags(fs *flag.FlagSet) {}

func (cmd *renameCommand) ParsePositional(fs *flag.FlagSet) error {
	return args.ExpectArgs(fs, &cmd.current, &cmd.new)
}

func isDefaultContainer(usrdata *userdata.Userdata, name string) (bool, error) {
	def, err := inventory.DefaultContainer(usrdata)
	if err != nil {
		return false, err
	}

	return def.Name == name, nil
}

func (cmd *renameCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	usrdata := app.(*nsboxApp).usrdata

	ct, err := container.Open(usrdata, cmd.current)
	if err != nil {
		return args.HandleError(err)
	}

	err = ct.LockUntilProcessDeath(container.FullContainerLock, container.NoWaitForLock)
	if err != nil {
		return args.HandleError(err)
	}

	isDefault, err := isDefaultContainer(usrdata, cmd.current)
	if err != nil {
		return args.HandleError(errors.Wrap(err, "checking default container"))
	}

	err = ct.Rename(cmd.new)
	if err != nil {
		if os.IsExist(err) {
			err = errors.New("new name already exists")
		}

		return args.HandleError(err)
	}

	if isDefault {
		// XXX: This is a bit racy, but in the worst-case scenario, the default container
		// ends up set to a now-deleted container.
		err = inventory.SetDefaultContainer(usrdata, cmd.new)
		if err != nil {
			return args.HandleError(err)
		}
	}

	return subcommands.ExitSuccess
}
