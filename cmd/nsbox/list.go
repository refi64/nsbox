/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"path/filepath"

	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/inventory"
	"github.com/refi64/nsbox/internal/log"
)

type listCommand struct {
	patterns []string
}

func newListCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &listCommand{})
}

func (*listCommand) Name() string {
	return "list"
}

func (*listCommand) Synopsis() string {
	return "list containers"
}

func (*listCommand) Usage() string {
	return `list [<patterns>...]:
	Lists all the available containers. If a pattern is given, list only containers whose names
	match one of the given patterns.
`
}

func (*listCommand) SetFlags(fs *flag.FlagSet) {}

func (cmd *listCommand) ParsePositional(fs *flag.FlagSet) error {
	cmd.patterns = fs.Args()
	return nil
}

func (cmd *listCommand) Execute(app args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	var containers []*container.Container
	containers, err := inventory.List(app.(*nsboxApp).usrdata)
	if err != nil {
		return args.HandleError(err)
	}

	for _, ct := range containers {
		if len(cmd.patterns) != 0 {
			var match bool

			for _, arg := range fs.Args() {
				match, err = filepath.Match(arg, ct.Name)
				if match {
					break
				}

				if err != nil {
					return args.HandleError(err)
				}
			}

			if !match {
				continue
			}
		}

		log.Info(ct.Name)
	}

	return subcommands.ExitSuccess
}
