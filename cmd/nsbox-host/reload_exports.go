/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package main

import (
	"context"
	"flag"

	"github.com/google/subcommands"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/args"
	devnsbox "github.com/refi64/nsbox/internal/varlink"
)

func reloadExports() error {
	conn, err := varlinkConnect()
	if err != nil {
		return err
	}

	defer conn.Close()

	if err := devnsbox.NotifyReloadExports().Call(context.Background(), conn); err != nil {
		return errors.Wrap(err, "failed to send reload exports message")
	}

	return nil
}

type reloadExportsCommand struct{}

func newReloadExportsCommand(app args.App) subcommands.Command {
	return args.WrapSimpleCommand(app, &reloadExportsCommand{})
}

func (*reloadExportsCommand) Name() string {
	return "reload-exports"
}

func (*reloadExportsCommand) Synopsis() string {
	return "reload all the files exported to the host."
}

func (*reloadExportsCommand) Usage() string {
	return `reload-exports:
	Reload all the files exported to the host.
`
}

func (*reloadExportsCommand) SetFlags(fs *flag.FlagSet) {
}

func (*reloadExportsCommand) ParsePositional(fs *flag.FlagSet) error {
	return args.ExpectArgs(fs)
}

func (*reloadExportsCommand) Execute(_ args.App, fs *flag.FlagSet) subcommands.ExitStatus {
	return args.HandleError(reloadExports())
}
