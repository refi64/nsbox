/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
)

type nsboxHostApp struct{}

func (app *nsboxHostApp) PreexecHook(fs *flag.FlagSet)    {}
func (app *nsboxHostApp) SetGlobalFlags(fs *flag.FlagSet) {}

func main() {
	app := &nsboxHostApp{}

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(newServiceCommand(app), "")
	subcommands.Register(newEnterCommand(app), "")
	subcommands.Register(newReloadExportsCommand(app), "")

	args.Execute(app)
}
