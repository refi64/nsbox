/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/integration"
	"golang.org/x/crypto/ssh/terminal"
)

type configCommand struct {
	name string

	extraBindMounts   args.ArrayTransformValue
	extraCapabilities args.ArrayTransformValue
	privateDirs       args.ArrayTransformValue
	shareDevices      args.ArrayTransformValue
	syscallFilters    args.ArrayTransformValue
	xdgDesktopExtra   args.ArrayTransformValue
	xdgDesktopExports args.ArrayTransformValue
	auth              container.Auth
	shareCgroupfs     bool
	virtualNetwork    bool
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
	fs.BoolVar(&cmd.shareCgroupfs, "share-cgroupfs", false, "share the host's cgroupfs")
	fs.BoolVar(&cmd.virtualNetwork, "virtual-network", false, "use a virtualized network")
	fs.Var(&cmd.auth, "auth", "password authentication method")
	fs.Var(&cmd.extraBindMounts, "extra-bind-mounts", "extra bind mounts")
	fs.Var(&cmd.extraCapabilities, "extra-capabilities", "extra capabilities to grant")
	fs.Var(&cmd.privateDirs, "private-dirs", "paths under home that will be private to the container")
	fs.Var(&cmd.shareDevices, "share-devices", "share devices with the container")
	fs.Var(&cmd.syscallFilters, "syscall-filters", "system call filters")
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

	if err := ct.LockUntilProcessDeath(container.ConfigLock, container.NoWaitForLock); err != nil {
		return args.HandleError(err)
	}

	fs.Visit(func(f *flag.Flag) {
		// XXX: This is ridiculous, all I want to know is if flags were actually given...
		if f.Name == "auth" {
			ct.Config.Auth = cmd.auth

			if cmd.auth == container.AuthManual {
				fmt.Print("Enter a password for the container user: ")

				pass, err2 := terminal.ReadPassword(int(os.Stdin.Fd()))
				if err2 != nil {
					err = err2
					return
				}

				fmt.Println()

				if err2 := ct.UpdateManualPassword(pass); err2 != nil {
					err = err2
					return
				}
			}
		} else if f.Name == "share-cgroupfs" {
			ct.Config.ShareCgroupfs = cmd.shareCgroupfs
		} else if f.Name == "virtual-network" {
			ct.Config.VirtualNetwork = cmd.virtualNetwork
		}
	})

	if err != nil {
		return args.HandleError(err)
	}

	cmd.extraBindMounts.Apply(&ct.Config.ExtraBindMounts)
	cmd.extraCapabilities.Apply(&ct.Config.ExtraCapabilities)
	cmd.privateDirs.Apply(&ct.Config.PrivateDirs)
	cmd.shareDevices.Apply(&ct.Config.ShareDevices)
	cmd.syscallFilters.Apply(&ct.Config.SyscallFilters)
	cmd.xdgDesktopExtra.Apply(&ct.Config.XdgDesktopExtra)
	cmd.xdgDesktopExports.Apply(&ct.Config.XdgDesktopExports)

	if err := ct.UpdateConfig(); err != nil {
		return args.HandleError(err)
	}

	return args.HandleError(integration.UpdateDesktopFiles(ct))
}
