/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"github.com/google/subcommands"
	"github.com/refi64/nsbox/internal/args"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/session"
)

type enterCommand struct {
	stdin string
	stdout string
	stderr string
	uid int
	cwd string
}

func newEnterCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &enterCommand{})
}

func (*enterCommand) Name() string {
	return "enter"
}

func (*enterCommand) Synopsis() string {
	return "INTERNAL COMMAND - enter a session"
}

func (*enterCommand) Usage() string {
	return "INTERNAL COMMAND - are you blind??"
}

func (cmd *enterCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&cmd.stdin, "stdin", "", "")
	fs.StringVar(&cmd.stdout, "stdout", "", "")
	fs.StringVar(&cmd.stderr, "stderr", "", "")
	fs.IntVar(&cmd.uid, "uid", -1, "")
	fs.StringVar(&cmd.cwd, "cwd", "", "")
}

func (cmd *enterCommand) Execute(fs *flag.FlagSet) subcommands.ExitStatus {
	if fs.NArg() == 0 {
		log.Alert("expected a command")
		return subcommands.ExitUsageError
	}

	if cmd.stdin == "" || cmd.stdout == "" || cmd.stderr == "" || cmd.cwd == "" || cmd.uid == -1 {
		log.Alert("missing arguments")
		return subcommands.ExitUsageError
	}

	err := session.ConnectPtys(cmd.stdin, cmd.stdout, cmd.stderr)
	if err == nil {
		err = session.SetupContainerSession(cmd.uid, cmd.cwd, fs.Args())
	}

	return args.HandleError(err)
}
