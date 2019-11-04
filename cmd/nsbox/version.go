/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/release"
)

type versionCommand struct {
}

func newVersionCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &versionCommand{})
}

func (*versionCommand) Name() string {
	return "version"
}

func (*versionCommand) Synopsis() string {
	return "show the nsbox version"
}

func (*versionCommand) Usage() string {
	return `version
	Show the current nsbox version.
`
}

func (*versionCommand) SetFlags(fs *flag.FlagSet) {}

func (cmd *versionCommand) ParsePositional(fs *flag.FlagSet) error {
	return nil
}

func (cmd *versionCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	rel, err := release.Read()
	if err != nil {
		return args.HandleError(err)
	}

	log.Infof("%s (%v)", rel.Version, rel.Branch)

	return args.HandleError(err)
}
