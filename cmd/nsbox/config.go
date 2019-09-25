/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/integration"
)

type configCommand struct {
	name string

	xdgDesktopExtra   args.ArrayTransformValue
	xdgDesktopExports args.ArrayTransformValue
}

func newConfigCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &configCommand{})
}

func (*configCommand) Name() string {
	return "config"
}

func (*configCommand) Synopsis() string {
	return "show container config"
}

func (*configCommand) Usage() string {
	return `config <container> [options]
	With no options given, prints the container configuration. If options are given, sets the
	container configurations corresponding to the given options.
`
}

func (cmd *configCommand) SetFlags(fs *flag.FlagSet) {
	fs.Var(&cmd.xdgDesktopExtra, "xdg-desktop-extra", "extra desktop file directories")
	fs.Var(&cmd.xdgDesktopExports, "xdg-desktop-exports", "exported desktop files patterns")
}

func (cmd *configCommand) ParsePositional(fs *flag.FlagSet) error {
	return args.ExpectArgs(fs, &cmd.name)
}

func (cmd *configCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	ct, err := container.Open(app.(*nsboxApp).usrdata, cmd.name)
	if err != nil {
		return args.HandleError(err)
	}

	if err := ct.LockUntilProcessDeath(container.NoWaitForLock); err != nil {
		return args.HandleError(err)
	}

	cmd.xdgDesktopExtra.Apply(&ct.Config.XdgDesktopExtra)
	cmd.xdgDesktopExports.Apply(&ct.Config.XdgDesktopExports)

	if err := ct.UpdateConfig(); err != nil {
		return args.HandleError(err)
	}

	return args.HandleError(integration.UpdateDesktopFiles(ct))
}
